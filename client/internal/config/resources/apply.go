package resources

import (
	"fmt"
	"strings"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

type proxyRecord struct {
	Name string
	Type string
}

func ValidatePlan(plan Plan, state State) (Plan, error) {
	normalized, err := NormalizePlan(plan)
	if err != nil {
		return Plan{}, err
	}
	if err := validatePlanReferences(normalized, state); err != nil {
		return Plan{}, err
	}
	return normalized, nil
}

// NormalizePlan validates the shape and merge modes without resolving IDs.
// Persistence uses this form; the application service separately checks the
// live library so missing resources produce a precise validation error.
func NormalizePlan(plan Plan) (Plan, error) {
	return normalizePlan(plan)
}

// RuleProviderCount returns the number of unique providers emitted by the
// explicitly ordered groups. Inline-expanded groups do not create providers.
func RuleProviderCount(input Plan, state State) (int, error) {
	plan, err := ValidatePlan(input, state)
	if err != nil {
		return 0, err
	}
	if plan.RulesDisabled {
		return 0, nil
	}
	groups := make(map[string]StrategyGroup, len(state.StrategyGroups))
	for _, group := range state.StrategyGroups {
		groups[group.ID] = group
	}
	providers := make(map[string]struct{})
	for _, id := range plan.StrategyGroupIDs {
		if !plan.GroupEnabled(id) {
			continue
		}
		group := groups[id]
		if group.RuleOutput != RuleOutputRuleSet {
			continue
		}
		for _, reference := range group.RuleSetReferences {
			providers[reference.RuleSetID] = struct{}{}
		}
	}
	return len(providers), nil
}

// Apply resolves reusable strategy groups against a composed subscription.
// Selectors become mihomo proxy-groups; rule and selector groups both emit
// ordered rules through RULE-SET or direct inline expansion.
func Apply(contents []byte, state State, input Plan) ([]byte, error) {
	plan, err := ValidatePlan(input, state)
	if err != nil {
		return nil, err
	}
	if plan.RulesDisabled {
		return contents, nil
	}
	if len(plan.StrategyGroupIDs) == 0 && plan.DefaultProxySelectorID == "" && plan.RuleStrategy != compose.StrategyReplace && plan.Match.Mode == "none" {
		return contents, nil
	}
	configuration, err := document.Parse(contents)
	if err != nil {
		return nil, fmt.Errorf("parse configuration for local resources: %w", err)
	}
	proxies, err := proxiesFromRoot(document.Root(configuration))
	if err != nil {
		return nil, err
	}
	groupsByID := make(map[string]StrategyGroup, len(state.StrategyGroups))
	for _, group := range state.StrategyGroups {
		groupsByID[group.ID] = group
	}
	ruleSetsByID := make(map[string]RuleSet, len(state.RuleSets))
	for _, ruleSet := range state.RuleSets {
		ruleSetsByID[ruleSet.ID] = ruleSet
	}

	explicitGroups := make([]StrategyGroup, 0, len(plan.StrategyGroupIDs))
	selectorIDs := make([]string, 0, len(plan.StrategyGroupIDs)+1)
	seenSelectors := make(map[string]struct{})
	addSelector := func(id string) {
		if id == "" || !plan.GroupEnabled(id) {
			return
		}
		if _, exists := seenSelectors[id]; exists {
			return
		}
		seenSelectors[id] = struct{}{}
		selectorIDs = append(selectorIDs, id)
	}
	for _, id := range plan.StrategyGroupIDs {
		if !plan.GroupEnabled(id) {
			continue
		}
		group := groupsByID[id]
		explicitGroups = append(explicitGroups, group)
		if group.Kind == StrategyGroupKindSelector {
			addSelector(group.ID)
		}
	}
	// Materialize the default selector only when enabled proxy rules need it.
	if hasProxyRules(explicitGroups) {
		addSelector(plan.DefaultProxySelectorID)
	}

	result := contents
	if len(selectorIDs) > 0 {
		selectors, materializeErr := materializeSelectors(selectorIDs, groupsByID, proxies)
		if materializeErr != nil {
			return nil, materializeErr
		}
		result, err = composeResourcePath(result, "/proxy-groups", selectors, plan.selectorStrategy(), true)
		if err != nil {
			return nil, fmt.Errorf("compose local selectors: %w", err)
		}
	}

	providers, generatedRules, err := materializeStrategyRules(
		explicitGroups,
		groupsByID,
		ruleSetsByID,
		plan.DefaultProxySelectorID,
		true,
	)
	if err != nil {
		return nil, err
	}
	if len(providers.Content) > 0 || plan.ruleProviderStrategy() == compose.StrategyReplace {
		result, err = composeResourcePath(result, "/rule-providers", providers, plan.ruleProviderStrategy(), true)
		if err != nil {
			return nil, fmt.Errorf("compose local rule providers: %w", err)
		}
	}
	if len(generatedRules) > 0 || plan.RuleStrategy == compose.StrategyReplace {
		result, err = composeRules(result, generatedRules, plan.RuleStrategy)
		if err != nil {
			return nil, fmt.Errorf("compose local strategy rules: %w", err)
		}
	}
	return applyLocalMatch(result, plan, groupsByID)
}

func composeResourcePath(contents []byte, path string, value *yaml.Node, strategy compose.Strategy, named bool) ([]byte, error) {
	overlay := document.New()
	if err := document.SetMappingPath(document.Root(overlay), path, value); err != nil {
		return nil, err
	}
	overlayYAML, err := document.Encode(overlay)
	if err != nil {
		return nil, err
	}
	conflict := compose.ConflictPolicy("")
	if named && strategy == compose.StrategyAppend {
		strategy = compose.StrategyMergeByName
		conflict = compose.ConflictError
	}
	composed, err := compose.Compose(contents, overlayYAML, []compose.Rule{{Path: path, Strategy: strategy, ConflictPolicy: conflict}})
	if err != nil {
		return nil, err
	}
	return []byte(composed.YAML), nil
}

func composeRules(contents []byte, generated []string, strategy compose.Strategy) ([]byte, error) {
	configuration, err := document.Parse(contents)
	if err != nil {
		return nil, err
	}
	root := document.Root(configuration)
	base, found, err := document.Find(root, "/rules")
	if err != nil {
		return nil, err
	}
	if !found {
		base = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	if base.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("configuration rules must be a sequence")
	}
	local := make([]*yaml.Node, 0, len(generated))
	for _, rule := range generated {
		local = append(local, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: rule})
	}
	terminalIndex := len(base.Content)
	for index, rule := range base.Content {
		if terminalRule(rule) {
			terminalIndex = index
			break
		}
	}
	combined := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	switch strategy {
	case compose.StrategyPrepend:
		appendClonedNodes(combined, local)
		appendClonedNodes(combined, base.Content)
	case compose.StrategyAppendBeforeTerminal:
		appendClonedNodes(combined, base.Content[:terminalIndex])
		appendClonedNodes(combined, local)
		appendClonedNodes(combined, base.Content[terminalIndex:])
	case compose.StrategyReplace:
		appendClonedNodes(combined, local)
	default:
		return nil, fmt.Errorf("unsupported local rule composition mode")
	}
	if err := document.SetMappingPath(root, "/rules", combined); err != nil {
		return nil, err
	}
	return document.Encode(configuration)
}

func appendClonedNodes(target *yaml.Node, values []*yaml.Node) {
	for _, value := range values {
		target.Content = append(target.Content, document.Clone(value))
	}
}

func terminalRule(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}
	kind := strings.ToUpper(strings.TrimSpace(strings.SplitN(node.Value, ",", 2)[0]))
	return kind == "MATCH" || kind == "FINAL"
}

func proxiesFromRoot(root *yaml.Node) ([]proxyRecord, error) {
	node, found, err := document.Find(root, "/proxies")
	if err != nil {
		return nil, err
	}
	if !found {
		return []proxyRecord{}, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("configuration proxies must be a sequence")
	}
	result := make([]proxyRecord, 0, len(node.Content))
	for index, proxy := range node.Content {
		if proxy.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("configuration proxy at index %d must be a mapping", index)
		}
		name := mappingScalar(proxy, "name")
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("configuration proxy at index %d has no name", index)
		}
		result = append(result, proxyRecord{Name: name, Type: strings.ToLower(mappingScalar(proxy, "type"))})
	}
	return result, nil
}

func materializeSelectors(ids []string, available map[string]StrategyGroup, proxies []proxyRecord) (*yaml.Node, error) {
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range ids {
		group, found := available[id]
		if !found || group.Kind != StrategyGroupKindSelector {
			return nil, fmt.Errorf("referenced local selector no longer exists")
		}
		members := make([]string, 0, len(proxies))
		seen := map[string]bool{}
		for _, proxy := range proxies {
			if !seen[proxy.Name] && matchesFilter(group.Filter, proxy) {
				members = append(members, proxy.Name)
				seen[proxy.Name] = true
			}
		}
		mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setScalar(mapping, "name", strategyGroupDisplayName(group))
		setScalar(mapping, "type", group.Type)
		if group.Icon != "" {
			setScalar(mapping, "icon", group.Icon)
		}
		setStringSequence(mapping, "proxies", members)
		if group.Type != "select" {
			setScalar(mapping, "url", group.URL)
			setInt(mapping, "interval", group.Interval)
			if group.Type == "url-test" && group.Tolerance > 0 {
				setInt(mapping, "tolerance", group.Tolerance)
			}
			if group.Type == "load-balance" {
				setScalar(mapping, "strategy", group.Strategy)
			}
			setBool(mapping, "lazy", group.Lazy)
		}
		sequence.Content = append(sequence.Content, mapping)
	}
	return sequence, nil
}

func materializeStrategyRules(
	groups []StrategyGroup,
	allGroups map[string]StrategyGroup,
	ruleSets map[string]RuleSet,
	proxySelectorID string,
	emitRules bool,
) (*yaml.Node, []string, error) {
	providers := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	providerNames := make(map[string]struct{})
	rules := make([]string, 0)
	for _, group := range groups {
		target := "DIRECT"
		if emitRules {
			var err error
			target, err = strategyRuleTarget(group, allGroups, proxySelectorID)
			if err != nil {
				return nil, nil, err
			}
		}
		for _, reference := range group.RuleSetReferences {
			ruleSet, found := ruleSets[reference.RuleSetID]
			if !found {
				return nil, nil, fmt.Errorf("referenced rule set no longer exists")
			}
			if group.RuleOutput == RuleOutputInline {
				expanded, expandErr := expandInlineRuleSet(ruleSet, target)
				if expandErr != nil {
					return nil, nil, fmt.Errorf("expand inline rule set %q: %w", ruleSet.Name, expandErr)
				}
				rules = append(rules, expanded...)
				continue
			}
			if _, exists := providerNames[ruleSet.Name]; !exists {
				providerNames[ruleSet.Name] = struct{}{}
				providers.Content = append(providers.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ruleSet.Name},
					materializeRuleProvider(ruleSet),
				)
			}
			rule := "RULE-SET," + ruleSet.Name + "," + target
			if ruleSet.NoResolve {
				rule += ",no-resolve"
			}
			rules = append(rules, rule)
		}
	}
	return providers, rules, nil
}

func strategyRuleTarget(group StrategyGroup, groups map[string]StrategyGroup, proxySelectorID string) (string, error) {
	if group.Kind == StrategyGroupKindSelector {
		return strategyGroupDisplayName(group), nil
	}
	switch group.Policy.Mode {
	case PolicyDirect:
		return "DIRECT", nil
	case PolicyReject:
		return "REJECT", nil
	case PolicyProxy:
		selector, found := groups[proxySelectorID]
		if !found || selector.Kind != StrategyGroupKindSelector {
			return "", fmt.Errorf("rule group %q requires an available default proxy selector", group.Name)
		}
		return strategyGroupDisplayName(selector), nil
	default:
		return "", fmt.Errorf("rule group %q has an unsupported policy", group.Name)
	}
}

func materializeRuleProvider(ruleSet RuleSet) *yaml.Node {
	provider := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	setScalar(provider, "type", ruleSet.SourceType)
	setScalar(provider, "behavior", ruleSet.Behavior)
	if ruleSet.SourceType == "http" {
		setScalar(provider, "format", ruleSet.Format)
		setScalar(provider, "url", ruleSet.URL)
		setScalar(provider, "path", "./rule-providers/"+ruleSet.ID+"."+ruleSet.Format)
		setInt(provider, "interval", ruleSet.Interval)
	} else {
		setStringSequence(provider, "payload", ruleSet.Payload)
	}
	return provider
}

func expandInlineRuleSet(ruleSet RuleSet, target string) ([]string, error) {
	if ruleSet.SourceType != "inline" {
		return nil, fmt.Errorf("remote rule sets cannot be expanded without fetching mutable content")
	}
	result := make([]string, 0, len(ruleSet.Payload))
	for _, raw := range ruleSet.Payload {
		expression := strings.TrimSpace(raw)
		if expression == "" {
			continue
		}
		switch ruleSet.Behavior {
		case "domain":
			kind := "DOMAIN"
			value := expression
			if strings.HasPrefix(value, "+.") {
				kind, value = "DOMAIN-SUFFIX", strings.TrimPrefix(value, "+.")
			} else if strings.HasPrefix(value, ".") {
				kind, value = "DOMAIN-SUFFIX", strings.TrimPrefix(value, ".")
			} else if strings.ContainsAny(value, "*?") {
				kind = "DOMAIN-WILDCARD"
			}
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("domain payload is empty")
			}
			result = append(result, kind+","+value+","+target)
		case "ipcidr":
			rule := "IP-CIDR," + expression + "," + target
			if ruleSet.NoResolve {
				rule += ",no-resolve"
			}
			result = append(result, rule)
		case "classical":
			kind := strings.ToUpper(strings.TrimSpace(strings.SplitN(expression, ",", 2)[0]))
			if kind == "MATCH" || kind == "FINAL" || kind == "RULE-SET" || kind == "SUB-RULE" {
				return nil, fmt.Errorf("classical inline payload cannot contain %s", kind)
			}
			hasNoResolve := strings.HasSuffix(strings.ToLower(expression), ",no-resolve")
			if hasNoResolve {
				expression = strings.TrimSpace(expression[:len(expression)-len(",no-resolve")])
			}
			rule := expression + "," + target
			if hasNoResolve {
				rule += ",no-resolve"
			}
			result = append(result, rule)
		default:
			return nil, fmt.Errorf("unsupported rule-set behavior")
		}
	}
	return result, nil
}

func matchesFilter(filter NodeFilter, proxy proxyRecord) bool {
	if len(filter.ProxyTypes) > 0 && !containsFold(filter.ProxyTypes, proxy.Type) {
		return false
	}
	includes := make([]string, 0)
	excludes := make([]string, 0)
	for _, pattern := range filter.NamePatterns {
		if strings.HasPrefix(pattern, "!") {
			excludes = append(excludes, strings.TrimSpace(strings.TrimPrefix(pattern, "!")))
			continue
		}
		includes = append(includes, pattern)
	}
	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if containsFoldText(proxy.Name, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	// Exclusions are intentionally applied last. The selector starts from all
	// subscription proxies, intersects the structured/positive filters, then
	// removes every name matched by an ! condition.
	for _, pattern := range excludes {
		if containsFoldText(proxy.Name, pattern) {
			return false
		}
	}
	return true
}

func containsFold(values []string, candidate string) bool {
	for _, value := range values {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

func containsFoldText(value, term string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(term)))
}

func mappingScalar(mapping *yaml.Node, key string) string {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key && mapping.Content[index+1].Kind == yaml.ScalarNode {
			return mapping.Content[index+1].Value
		}
	}
	return ""
}

func setScalar(mapping *yaml.Node, key, value string) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

func setInt(mapping *yaml.Node, key string, value int) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%d", value)},
	)
}

func setBool(mapping *yaml.Node, key string, value bool) {
	encoded := "false"
	if value {
		encoded = "true"
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: encoded},
	)
}

func setStringSequence(mapping *yaml.Node, key string, values []string) {
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, sequence,
	)
}
