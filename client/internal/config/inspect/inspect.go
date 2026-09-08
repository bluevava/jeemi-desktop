package inspect

import (
	"fmt"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

type Member struct {
	Name         string
	Type         string
	Source       string
	ProviderName string
}

type Selector struct {
	Name                    string
	Icon                    string
	Hidden                  bool
	Type                    string
	DefaultSelection        string
	Members                 []Member
	ProviderNames           []string
	UnresolvedProviderNames []string
	ReferencedByRules       bool
}

type RuleProvider struct {
	Name     string
	Type     string
	Behavior string
	Format   string
}

type Result struct {
	ProxyCount         int
	ProxyProviderCount int
	Proxies            []Member
	Selectors          []Selector
	RuleProviders      []RuleProvider
	Warnings           []string
}

type proxyRecord struct {
	Name string
	Type string
}

type proxyProvider struct {
	Name    string
	Type    string
	Payload []proxyRecord
}

// RuleProviders returns the configured rule-provider definitions without
// evaluating selector membership or fetching any remote provider resource.
func RuleProviders(contents []byte) ([]RuleProvider, error) {
	parsed, err := document.Parse(contents)
	if err != nil {
		return nil, err
	}
	return ruleProvidersFromNode(mappingValue(document.Root(parsed), "rule-providers"))
}

// Configuration returns a side-effect-free UI projection of a composed mihomo
// document. Remote provider content and mihomo health data are intentionally
// not fetched or simulated here.
func Configuration(contents []byte) (Result, error) {
	parsed, err := document.Parse(contents)
	if err != nil {
		return Result{}, err
	}
	root := document.Root(parsed)
	proxies, err := namedSequence(mappingValue(root, "proxies"), "proxies")
	if err != nil {
		return Result{}, err
	}
	providers, err := proxyProviders(mappingValue(root, "proxy-providers"))
	if err != nil {
		return Result{}, err
	}
	groupsNode := mappingValue(root, "proxy-groups")
	groupNames, err := groupNameSet(groupsNode)
	if err != nil {
		return Result{}, err
	}
	selectors, warnings, err := selectorsFromNode(groupsNode, proxies, providers, groupNames)
	if err != nil {
		return Result{}, err
	}
	ruleTargets := referencedRuleTargets(root, groupNames)
	for index := range selectors {
		_, selectors[index].ReferencedByRules = ruleTargets[selectors[index].Name]
	}
	ruleProviders, err := ruleProvidersFromNode(mappingValue(root, "rule-providers"))
	if err != nil {
		return Result{}, err
	}
	proxyNames := make(map[string]struct{}, len(proxies))
	for _, proxy := range proxies {
		proxyNames[proxy.Name] = struct{}{}
	}
	for _, provider := range providers {
		for _, proxy := range provider.Payload {
			proxyNames[proxy.Name] = struct{}{}
		}
	}
	return Result{
		ProxyCount:         len(proxyNames),
		ProxyProviderCount: len(providers),
		Proxies:            flattenedProxies(proxies, providers),
		Selectors:          selectors,
		RuleProviders:      ruleProviders,
		Warnings:           warnings,
	}, nil
}

func flattenedProxies(proxies []proxyRecord, providers []proxyProvider) []Member {
	result := make([]Member, 0, len(proxies))
	seen := make(map[string]struct{})
	appendProxy := func(proxy proxyRecord, source, providerName string) {
		if proxy.Name == "" {
			return
		}
		if _, exists := seen[proxy.Name]; exists {
			return
		}
		seen[proxy.Name] = struct{}{}
		result = append(result, Member{
			Name: proxy.Name, Type: proxy.Type, Source: source, ProviderName: providerName,
		})
	}
	for _, proxy := range proxies {
		appendProxy(proxy, "proxy", "")
	}
	for _, provider := range providers {
		for _, proxy := range provider.Payload {
			appendProxy(proxy, "provider", provider.Name)
		}
	}
	return result
}

// referencedRuleTargets returns only names that are both a rule target and a
// configured proxy group. Rule expressions are split on top-level commas so
// logical AND/OR expressions keep their nested commas intact.
func referencedRuleTargets(root *yaml.Node, groupNames map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	collectRuleTargets(mappingValue(root, "rules"), groupNames, result)
	subRules := mappingValue(root, "sub-rules")
	if subRules != nil && subRules.Kind == yaml.MappingNode {
		for index := 1; index < len(subRules.Content); index += 2 {
			collectRuleTargets(subRules.Content[index], groupNames, result)
		}
	}
	return result
}

func collectRuleTargets(node *yaml.Node, groupNames map[string]struct{}, result map[string]struct{}) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range node.Content {
		parts := splitRuleFields(scalar(item))
		if len(parts) < 2 {
			continue
		}
		targetIndex := len(parts) - 1
		if strings.EqualFold(parts[targetIndex], "no-resolve") {
			targetIndex--
		}
		if targetIndex < 1 {
			continue
		}
		target := strings.TrimSpace(parts[targetIndex])
		if _, exists := groupNames[target]; exists {
			result[target] = struct{}{}
		}
	}
}

func splitRuleFields(rule string) []string {
	if rule == "" {
		return nil
	}
	fields := make([]string, 0, 4)
	start, depth := 0, 0
	for index, character := range rule {
		switch character {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				fields = append(fields, strings.TrimSpace(rule[start:index]))
				start = index + 1
			}
		}
	}
	fields = append(fields, strings.TrimSpace(rule[start:]))
	return fields
}

func selectorsFromNode(node *yaml.Node, proxies []proxyRecord, providers []proxyProvider, groupNames map[string]struct{}) ([]Selector, []string, error) {
	if node == nil {
		return []Selector{}, []string{}, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, nil, fmt.Errorf("proxy-groups must be a sequence")
	}
	proxyTypes := make(map[string]string, len(proxies))
	for _, proxy := range proxies {
		proxyTypes[proxy.Name] = proxy.Type
	}
	providerByName := make(map[string]proxyProvider, len(providers))
	for _, provider := range providers {
		providerByName[provider.Name] = provider
	}
	result := make([]Selector, 0, len(node.Content))
	warnings := make([]string, 0)
	for _, group := range node.Content {
		if group.Kind != yaml.MappingNode {
			return nil, nil, fmt.Errorf("proxy group must be a mapping")
		}
		name := scalar(mappingValue(group, "name"))
		if name == "" {
			return nil, nil, fmt.Errorf("proxy group has no name")
		}
		selector := Selector{
			Name:   name,
			Icon:   scalar(mappingValue(group, "icon")),
			Hidden: truthy(mappingValue(group, "hidden")),
			Type:   scalar(mappingValue(group, "type")),
		}
		seenMembers := make(map[string]struct{})
		appendMember := func(member Member) {
			identity := member.Source + "\x00" + member.ProviderName + "\x00" + member.Name
			if member.Name == "" {
				return
			}
			if _, exists := seenMembers[identity]; exists {
				return
			}
			seenMembers[identity] = struct{}{}
			selector.Members = append(selector.Members, member)
		}
		membersNode := mappingValue(group, "proxies")
		if membersNode != nil {
			if membersNode.Kind != yaml.SequenceNode {
				return nil, nil, fmt.Errorf("proxy group %q proxies must be a sequence", name)
			}
			for _, memberNode := range membersNode.Content {
				memberName := scalar(memberNode)
				memberType, source := proxyTypes[memberName], "proxy"
				if _, exists := groupNames[memberName]; exists {
					memberType, source = "group", "group"
				} else if memberType == "" {
					memberType, source = "builtin", "builtin"
				}
				appendMember(Member{Name: memberName, Type: memberType, Source: source})
			}
		}
		providerNames, err := scalarSequence(mappingValue(group, "use"))
		if err != nil {
			return nil, nil, fmt.Errorf("proxy group %q use: %w", name, err)
		}
		if truthy(mappingValue(group, "include-all-providers")) {
			for _, provider := range providers {
				providerNames = appendUnique(providerNames, provider.Name)
			}
		}
		selector.ProviderNames = providerNames
		for _, providerName := range providerNames {
			provider, exists := providerByName[providerName]
			if !exists || len(provider.Payload) == 0 {
				selector.UnresolvedProviderNames = append(selector.UnresolvedProviderNames, providerName)
				continue
			}
			for _, proxy := range provider.Payload {
				appendMember(Member{Name: proxy.Name, Type: proxy.Type, Source: "provider", ProviderName: providerName})
			}
		}
		if truthy(mappingValue(group, "include-all-proxies")) {
			for _, proxy := range proxies {
				appendMember(Member{Name: proxy.Name, Type: proxy.Type, Source: "proxy"})
			}
		}
		if mappingValue(group, "filter") != nil || mappingValue(group, "exclude-filter") != nil || mappingValue(group, "exclude-type") != nil {
			warnings = appendUnique(warnings, "selector_dynamic_filter_not_evaluated")
		}
		if len(selector.Members) > 0 {
			selector.DefaultSelection = selector.Members[0].Name
		}
		result = append(result, selector)
	}
	return result, warnings, nil
}

func proxyProviders(node *yaml.Node) ([]proxyProvider, error) {
	if node == nil {
		return []proxyProvider{}, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("proxy-providers must be a mapping")
	}
	result := make([]proxyProvider, 0, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		name := node.Content[index].Value
		value := node.Content[index+1]
		if value.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("proxy provider %q must be a mapping", name)
		}
		payload, err := namedSequence(mappingValue(value, "payload"), "proxy provider payload")
		if err != nil {
			return nil, err
		}
		result = append(result, proxyProvider{Name: name, Type: scalar(mappingValue(value, "type")), Payload: payload})
	}
	return result, nil
}

func ruleProvidersFromNode(node *yaml.Node) ([]RuleProvider, error) {
	if node == nil {
		return []RuleProvider{}, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("rule-providers must be a mapping")
	}
	result := make([]RuleProvider, 0, len(node.Content)/2)
	for index := 0; index < len(node.Content); index += 2 {
		name := node.Content[index].Value
		value := node.Content[index+1]
		if value.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("rule provider %q must be a mapping", name)
		}
		result = append(result, RuleProvider{
			Name: name, Type: scalar(mappingValue(value, "type")),
			Behavior: scalar(mappingValue(value, "behavior")), Format: scalar(mappingValue(value, "format")),
		})
	}
	return result, nil
}

func namedSequence(node *yaml.Node, label string) ([]proxyRecord, error) {
	if node == nil {
		return []proxyRecord{}, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be a sequence", label)
	}
	result := make([]proxyRecord, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s item must be a mapping", label)
		}
		name := scalar(mappingValue(item, "name"))
		if name == "" {
			return nil, fmt.Errorf("%s item has no name", label)
		}
		result = append(result, proxyRecord{Name: name, Type: scalar(mappingValue(item, "type"))})
	}
	return result, nil
}

func groupNameSet(node *yaml.Node) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if node == nil {
		return result, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("proxy-groups must be a sequence")
	}
	for _, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("proxy group must be a mapping")
		}
		name := scalar(mappingValue(item, "name"))
		if name == "" {
			return nil, fmt.Errorf("proxy group has no name")
		}
		result[name] = struct{}{}
	}
	return result, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func scalar(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(node.Value)
}

func scalarSequence(node *yaml.Node) ([]string, error) {
	if node == nil {
		return []string{}, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("must be a sequence")
	}
	result := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		value := scalar(item)
		if value == "" {
			return nil, fmt.Errorf("contains a non-scalar name")
		}
		result = appendUnique(result, value)
	}
	return result, nil
}

func truthy(node *yaml.Node) bool {
	return strings.EqualFold(scalar(node), "true")
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
