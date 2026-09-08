package resources

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	maxResourceNameLength        = 80
	maxResourceDescriptionLength = 500
	maxFilterValues              = 64
	maxNamePatterns              = 32
	maxNamePatternLength         = 64
	maxPayloadRules              = 20000
)

var (
	resourceIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
	proxyTypePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

func normalizeStrategyGroup(input StrategyGroup, now time.Time) (StrategyGroup, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	input.RuleOutput = strings.ToLower(strings.TrimSpace(input.RuleOutput))
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.Emoji = strings.TrimSpace(input.Emoji)
	input.Icon = strings.TrimSpace(input.Icon)
	input.URL = strings.TrimSpace(input.URL)
	input.Strategy = strings.ToLower(strings.TrimSpace(input.Strategy))
	if input.Name == "" || len([]rune(input.Name)) > maxResourceNameLength {
		return StrategyGroup{}, fmt.Errorf("strategy group name is required and must be at most %d characters", maxResourceNameLength)
	}
	if len([]rune(input.Description)) > maxResourceDescriptionLength {
		return StrategyGroup{}, fmt.Errorf("strategy group description is too long")
	}
	if !contains([]string{StrategyGroupKindRule, StrategyGroupKindSelector}, input.Kind) {
		return StrategyGroup{}, fmt.Errorf("strategy group kind is unsupported")
	}
	if input.RuleOutput == "" {
		input.RuleOutput = RuleOutputRuleSet
	}
	if !contains([]string{RuleOutputRuleSet, RuleOutputInline}, input.RuleOutput) {
		return StrategyGroup{}, fmt.Errorf("strategy group rule output is unsupported")
	}
	if input.RuleSetReferences == nil {
		input.RuleSetReferences = []RuleSetReference{}
	}
	seenRuleSets := make(map[string]struct{}, len(input.RuleSetReferences))
	for index := range input.RuleSetReferences {
		reference := &input.RuleSetReferences[index]
		if !resourceIDPattern.MatchString(reference.RuleSetID) {
			return StrategyGroup{}, fmt.Errorf("strategy group rule set reference is invalid")
		}
		if _, duplicate := seenRuleSets[reference.RuleSetID]; duplicate {
			return StrategyGroup{}, fmt.Errorf("strategy group rule set reference is duplicated")
		}
		seenRuleSets[reference.RuleSetID] = struct{}{}
	}

	if input.Emoji != "" {
		if _, allowed := selectorEmojiSet[input.Emoji]; !allowed {
			return StrategyGroup{}, fmt.Errorf("strategy group emoji must use a supported option")
		}
	}
	if input.Kind == StrategyGroupKindRule {
		if len(input.RuleSetReferences) == 0 {
			return StrategyGroup{}, fmt.Errorf("rule group must reference at least one rule set")
		}
		input.Policy.Mode = strings.ToLower(strings.TrimSpace(input.Policy.Mode))
		if !contains([]string{PolicyProxy, PolicyDirect, PolicyReject}, input.Policy.Mode) {
			return StrategyGroup{}, fmt.Errorf("rule group policy is unsupported")
		}
		input.Type = ""
		input.Icon = ""
		input.URL = ""
		input.Interval = 0
		input.Tolerance = 0
		input.Strategy = ""
		input.Lazy = false
		input.Filter = NodeFilter{ProxyTypes: []string{}, NamePatterns: []string{}}
	} else {
		input.Policy = PolicyTarget{}
		if !contains([]string{"select", "url-test", "fallback", "load-balance"}, input.Type) {
			return StrategyGroup{}, fmt.Errorf("selector type is unsupported")
		}
		if input.Icon != "" && !isHTTPURL(input.Icon) {
			return StrategyGroup{}, fmt.Errorf("selector icon must use http or https")
		}
		if input.Type == "select" {
			input.URL = ""
			input.Interval = 0
			input.Tolerance = 0
			input.Strategy = ""
			input.Lazy = false
		} else {
			if input.URL == "" {
				input.URL = defaultSelectorTestURL
			}
			if !isHTTPURL(input.URL) {
				return StrategyGroup{}, fmt.Errorf("selector test URL must use http or https")
			}
			if input.Interval == 0 {
				input.Interval = 300
			}
			if input.Interval < 10 || input.Interval > 86400 {
				return StrategyGroup{}, fmt.Errorf("selector interval must be between 10 and 86400 seconds")
			}
			if input.Type == "url-test" {
				if input.Tolerance < 0 || input.Tolerance > 60000 {
					return StrategyGroup{}, fmt.Errorf("selector tolerance is invalid")
				}
			} else {
				input.Tolerance = 0
			}
			if input.Type == "load-balance" {
				if input.Strategy == "" {
					input.Strategy = "consistent-hashing"
				}
				if !contains([]string{"round-robin", "consistent-hashing", "sticky-sessions"}, input.Strategy) {
					return StrategyGroup{}, fmt.Errorf("load balance strategy is unsupported")
				}
			} else {
				input.Strategy = ""
			}
		}
		filter, err := normalizeFilter(input.Filter)
		if err != nil {
			return StrategyGroup{}, err
		}
		input.Filter = filter
	}
	if input.CreatedAt == "" {
		input.CreatedAt = now.Format(time.RFC3339Nano)
	}
	input.UpdatedAt = now.Format(time.RFC3339Nano)
	return input, nil
}

func normalizeFilter(input NodeFilter) (NodeFilter, error) {
	if len(input.ProxyTypes) > maxFilterValues || len(input.NamePatterns) > maxNamePatterns {
		return NodeFilter{}, fmt.Errorf("strategy group filter has too many values")
	}
	result := NodeFilter{ProxyTypes: []string{}, NamePatterns: []string{}}
	seen := make(map[string]struct{})
	for _, value := range input.ProxyTypes {
		value = strings.ToLower(strings.TrimSpace(value))
		if !proxyTypePattern.MatchString(value) {
			return NodeFilter{}, fmt.Errorf("proxy type filter %q is invalid", value)
		}
		if _, exists := seen["type:"+value]; !exists {
			seen["type:"+value] = struct{}{}
			result.ProxyTypes = append(result.ProxyTypes, value)
		}
	}
	for _, value := range input.NamePatterns {
		value = strings.TrimSpace(value)
		term := strings.TrimSpace(strings.TrimPrefix(value, "!"))
		if term == "" || len([]rune(term)) > maxNamePatternLength {
			return NodeFilter{}, fmt.Errorf("proxy name filter must contain 1-%d characters", maxNamePatternLength)
		}
		key := "name:" + strings.ToLower(value)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result.NamePatterns = append(result.NamePatterns, value)
		}
	}
	sort.Strings(result.ProxyTypes)
	return result, nil
}

func normalizeRuleSet(input RuleSet, now time.Time) (RuleSet, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.SourceType = strings.ToLower(strings.TrimSpace(input.SourceType))
	input.Behavior = strings.ToLower(strings.TrimSpace(input.Behavior))
	input.Format = strings.ToLower(strings.TrimSpace(input.Format))
	input.URL = strings.TrimSpace(input.URL)
	if input.Name == "" || len([]rune(input.Name)) > maxResourceNameLength {
		return RuleSet{}, fmt.Errorf("rule set name is required and must be at most %d characters", maxResourceNameLength)
	}
	if len([]rune(input.Description)) > maxResourceDescriptionLength {
		return RuleSet{}, fmt.Errorf("rule set description is too long")
	}
	if !contains([]string{"inline", "http"}, input.SourceType) {
		return RuleSet{}, fmt.Errorf("rule set source type is unsupported")
	}
	if !contains([]string{"domain", "ipcidr", "classical"}, input.Behavior) {
		return RuleSet{}, fmt.Errorf("rule set behavior is unsupported")
	}
	if input.Behavior != "ipcidr" {
		input.NoResolve = false
	}
	if input.SourceType == "http" {
		if !contains([]string{"yaml", "text", "mrs"}, input.Format) {
			return RuleSet{}, fmt.Errorf("remote rule set format is unsupported")
		}
		if !isHTTPURL(input.URL) {
			return RuleSet{}, fmt.Errorf("remote rule set URL must use http or https")
		}
		if input.Interval == 0 {
			input.Interval = 86400
		}
		if input.Interval < 60 || input.Interval > 604800 {
			return RuleSet{}, fmt.Errorf("rule set interval must be between 60 and 604800 seconds")
		}
		input.Payload = []string{}
		input.PayloadYAML = ""
	} else {
		input.Format = "yaml"
		input.URL = ""
		input.Interval = 0
		payloadYAML, payload, err := normalizeRuleSetPayloadYAML(input.PayloadYAML)
		if err != nil {
			return RuleSet{}, err
		}
		if err := validateRuleSetPayloadRules(input.Behavior, payload); err != nil {
			return RuleSet{}, err
		}
		input.PayloadYAML = payloadYAML
		input.Payload = payload
	}
	if input.CreatedAt == "" {
		input.CreatedAt = now.Format(time.RFC3339Nano)
	}
	input.UpdatedAt = now.Format(time.RFC3339Nano)
	return input, nil
}

func normalizePlan(input Plan) (Plan, error) {
	result := input
	if result.Version == 0 {
		result.Version = CurrentPlanVersion
	}
	if result.Version != CurrentPlanVersion {
		return Plan{}, fmt.Errorf("local rule plan version is unsupported")
	}
	if result.Match.Mode == "" {
		result.Match.Mode = "none"
	}
	if result.Match.Mode != "none" && result.Match.Mode != "direct" && result.Match.Mode != "selector" {
		return Plan{}, fmt.Errorf("local MATCH mode is unsupported")
	}
	if result.Match.Mode == "selector" {
		if !resourceIDPattern.MatchString(result.Match.SelectorID) {
			return Plan{}, fmt.Errorf("local MATCH selector reference is invalid")
		}
	} else if result.Match.SelectorID != "" {
		return Plan{}, fmt.Errorf("local MATCH selector must be empty for this mode")
	}
	if result.StrategyGroupIDs == nil {
		result.StrategyGroupIDs = []string{}
	}
	if result.DisabledStrategyGroupIDs == nil {
		result.DisabledStrategyGroupIDs = []string{}
	}
	if result.RuleStrategy == "" {
		result.RuleStrategy = composeStrategyAppendBeforeTerminal()
	}
	if !contains([]string{string(composeStrategyPrepend()), string(composeStrategyAppendBeforeTerminal()), string(composeStrategyReplace())}, string(result.RuleStrategy)) {
		return Plan{}, fmt.Errorf("rule composition mode is unsupported")
	}
	seen := make(map[string]struct{})
	for _, id := range result.StrategyGroupIDs {
		if !resourceIDPattern.MatchString(id) {
			return Plan{}, fmt.Errorf("strategy group reference is invalid")
		}
		if _, duplicate := seen["group:"+id]; duplicate {
			return Plan{}, fmt.Errorf("strategy group reference is duplicated")
		}
		seen["group:"+id] = struct{}{}
	}
	for _, id := range result.DisabledStrategyGroupIDs {
		if _, found := seen["group:"+id]; !found {
			return Plan{}, fmt.Errorf("disabled strategy group must be included in this configuration")
		}
		if _, duplicate := seen["disabled:"+id]; duplicate {
			return Plan{}, fmt.Errorf("disabled strategy group reference is duplicated")
		}
		seen["disabled:"+id] = struct{}{}
	}
	result.DefaultProxySelectorID = strings.TrimSpace(result.DefaultProxySelectorID)
	if result.DefaultProxySelectorID != "" && !resourceIDPattern.MatchString(result.DefaultProxySelectorID) {
		return Plan{}, fmt.Errorf("default proxy selector reference is invalid")
	}
	return result, nil
}

func validatePlanReferences(plan Plan, state State) error {
	if plan.RulesDisabled {
		return nil
	}
	groups := make(map[string]StrategyGroup, len(state.StrategyGroups))
	for _, item := range state.StrategyGroups {
		groups[item.ID] = item
	}
	needsProxySelector := false
	hasSelector := false
	for _, id := range plan.StrategyGroupIDs {
		if !plan.GroupEnabled(id) {
			continue
		}
		group, found := groups[id]
		if !found {
			return fmt.Errorf("referenced strategy group no longer exists")
		}
		if group.Kind == StrategyGroupKindSelector {
			hasSelector = true
		}
		if group.Kind == StrategyGroupKindRule && group.Policy.Mode == PolicyProxy {
			needsProxySelector = true
		}
	}
	if plan.RuleStrategy == composeStrategyReplace() && !hasSelector {
		return fmt.Errorf("replacing subscription selectors requires an explicitly included local selector")
	}
	if plan.RuleStrategy == composeStrategyReplace() && plan.Match.Mode == "none" {
		return fmt.Errorf("complete rule reconstruction requires a local MATCH target")
	}
	if plan.Match.Mode == "selector" {
		selector, found := groups[plan.Match.SelectorID]
		if !found || selector.Kind != StrategyGroupKindSelector || !contains(plan.StrategyGroupIDs, selector.ID) || !plan.GroupEnabled(selector.ID) {
			return fmt.Errorf("local MATCH must reference an included, enabled local selector")
		}
	}
	if needsProxySelector && plan.DefaultProxySelectorID == "" {
		return fmt.Errorf("proxy-directed rules require a default proxy selector")
	}
	if needsProxySelector && !plan.GroupEnabled(plan.DefaultProxySelectorID) {
		return fmt.Errorf("proxy-directed rules reference a disabled selector; enable it or choose another proxy target")
	}
	if needsProxySelector && plan.DefaultProxySelectorID != "" && plan.GroupEnabled(plan.DefaultProxySelectorID) {
		selector, found := groups[plan.DefaultProxySelectorID]
		if !found || selector.Kind != StrategyGroupKindSelector {
			return fmt.Errorf("default proxy selector is unavailable")
		}
	}
	return nil
}

func validateStrategyGroupReferences(group StrategyGroup, state State) error {
	ruleSets := make(map[string]RuleSet, len(state.RuleSets))
	for _, item := range state.RuleSets {
		ruleSets[item.ID] = item
	}
	for _, reference := range group.RuleSetReferences {
		ruleSet, found := ruleSets[reference.RuleSetID]
		if !found {
			return fmt.Errorf("strategy group references an unavailable rule set")
		}
		if group.RuleOutput == RuleOutputInline && ruleSet.SourceType != "inline" {
			return fmt.Errorf("inline rule output requires locally editable rule-set payload")
		}
	}
	return nil
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func isHTTPURL(candidate string) bool {
	parsed, err := url.Parse(candidate)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
