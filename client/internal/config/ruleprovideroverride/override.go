// Package ruleprovideroverride applies Jeemi's per-subscription rule-provider
// switches after subscription/local composition and before runtime-owned
// fields are injected. It never mutates the saved subscription revision.
package ruleprovideroverride

import (
	"fmt"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

// Apply removes disabled rule providers and every known mihomo configuration
// reference that would otherwise point at a provider which no longer exists.
func Apply(configurationYAML []byte, disabledNames []string) ([]byte, error) {
	disabled := nameSet(disabledNames)
	if len(disabled) == 0 {
		return append([]byte(nil), configurationYAML...), nil
	}

	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse composed configuration: %w", err)
	}
	root := document.Root(configuration)
	providers := mappingValue(root, "rule-providers")
	if providers != nil {
		if providers.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("rule-providers must be a mapping")
		}
		filterMappingEntries(providers, func(key string, _ *yaml.Node) (string, bool) {
			_, remove := disabled[strings.TrimSpace(key)]
			return key, !remove
		})
	}

	if err := filterRuleSequence(mappingValue(root, "rules"), disabled, "rules"); err != nil {
		return nil, err
	}
	if err := filterSubRules(mappingValue(root, "sub-rules"), disabled); err != nil {
		return nil, err
	}
	if err := filterKnownReferences(root, disabled); err != nil {
		return nil, err
	}

	encoded, err := document.Encode(configuration)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func filterSubRules(node *yaml.Node, disabled map[string]struct{}) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("sub-rules must be a mapping")
	}
	for index := 1; index < len(node.Content); index += 2 {
		if err := filterRuleSequence(node.Content[index], disabled, "sub-rules"); err != nil {
			return err
		}
	}
	return nil
}

func filterKnownReferences(node *yaml.Node, disabled map[string]struct{}) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index].Value
			value := node.Content[index+1]
			switch key {
			case "nameserver-policy", "proxy-server-nameserver-policy":
				if err := filterNameServerPolicy(value, disabled); err != nil {
					return err
				}
			case "fake-ip-filter":
				if err := filterFakeIPFilter(value, disabled); err != nil {
					return err
				}
			case "force-domain", "skip-domain", "skip-src-address", "skip-dst-address":
				if err := filterRuleSetReferenceSequence(value, disabled, key); err != nil {
					return err
				}
			case "route-address-set", "route-exclude-address-set":
				if err := filterExactNameSequence(value, disabled, key); err != nil {
					return err
				}
			}
			if err := filterKnownReferences(value, disabled); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := filterKnownReferences(child, disabled); err != nil {
				return err
			}
		}
	}
	return nil
}

func filterFakeIPFilter(node *yaml.Node, disabled map[string]struct{}) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("fake-ip-filter must be a sequence")
	}
	filtered := node.Content[:0]
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return fmt.Errorf("fake-ip-filter entries must be scalar strings")
		}
		if ruleReferencesDisabled(item.Value, disabled) {
			continue
		}
		next, keep := filterRuleSetReference(item.Value, disabled)
		if keep {
			item.Value = next
			filtered = append(filtered, item)
		}
	}
	node.Content = filtered
	return nil
}

func filterRuleSequence(node *yaml.Node, disabled map[string]struct{}, label string) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a sequence", label)
	}
	filtered := node.Content[:0]
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s entries must be scalar rule strings", label)
		}
		if !ruleReferencesDisabled(item.Value, disabled) {
			filtered = append(filtered, item)
		}
	}
	node.Content = filtered
	return nil
}

func ruleReferencesDisabled(rule string, disabled map[string]struct{}) bool {
	fields := strings.Split(rule, ",")
	for index := 0; index+1 < len(fields); index++ {
		kind := strings.Trim(strings.TrimSpace(fields[index]), "()")
		if !strings.EqualFold(kind, "RULE-SET") {
			continue
		}
		name := strings.Trim(strings.TrimSpace(fields[index+1]), "()")
		if _, found := disabled[name]; found {
			return true
		}
	}
	return false
}

func filterNameServerPolicy(node *yaml.Node, disabled map[string]struct{}) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("nameserver-policy must be a mapping")
	}
	seen := make(map[string]struct{}, len(node.Content)/2)
	filterMappingEntries(node, func(key string, _ *yaml.Node) (string, bool) {
		next, keep := filterRuleSetReference(key, disabled)
		if !keep {
			return key, false
		}
		identity := strings.ToLower(next)
		if _, duplicate := seen[identity]; duplicate {
			return next, false
		}
		seen[identity] = struct{}{}
		return next, true
	})
	return nil
}

func filterRuleSetReferenceSequence(node *yaml.Node, disabled map[string]struct{}, label string) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a sequence", label)
	}
	filtered := node.Content[:0]
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s entries must be scalar strings", label)
		}
		next, keep := filterRuleSetReference(item.Value, disabled)
		if keep {
			item.Value = next
			filtered = append(filtered, item)
		}
	}
	node.Content = filtered
	return nil
}

func filterRuleSetReference(value string, disabled map[string]struct{}) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < len("rule-set:") || !strings.EqualFold(trimmed[:len("rule-set:")], "rule-set:") {
		return value, true
	}
	names := strings.Split(trimmed[len("rule-set:"):], ",")
	kept := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, remove := disabled[name]; !remove {
			kept = append(kept, name)
		}
	}
	if len(kept) == 0 {
		return "", false
	}
	return "rule-set:" + strings.Join(kept, ","), true
}

func filterExactNameSequence(node *yaml.Node, disabled map[string]struct{}, label string) error {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a sequence", label)
	}
	filtered := node.Content[:0]
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s entries must be scalar names", label)
		}
		if _, remove := disabled[strings.TrimSpace(item.Value)]; !remove {
			filtered = append(filtered, item)
		}
	}
	node.Content = filtered
	return nil
}

func filterMappingEntries(node *yaml.Node, keep func(string, *yaml.Node) (string, bool)) {
	filtered := node.Content[:0]
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index]
		value := node.Content[index+1]
		nextKey, include := keep(key.Value, value)
		if !include {
			continue
		}
		key.Value = nextKey
		filtered = append(filtered, key, value)
	}
	node.Content = filtered
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

func nameSet(names []string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}
