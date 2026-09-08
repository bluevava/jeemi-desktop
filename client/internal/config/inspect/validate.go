package inspect

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
)

// ValidateReferences checks the static graph without resolving remote content.
// It supplements, rather than replaces, validation by the selected mihomo.
func ValidateReferences(contents []byte) error {
	if _, err := Configuration(contents); err != nil {
		return err
	}
	parsed, err := document.Parse(contents)
	if err != nil {
		return err
	}
	root := document.Root(parsed)
	targets := map[string]bool{"DIRECT": true, "REJECT": true, "REJECT-DROP": true, "PASS": true, "COMPATIBLE": true, "GLOBAL": true}
	for _, path := range []string{"proxies", "proxy-groups"} {
		if entries := mappingValue(root, path); entries != nil {
			for _, entry := range entries.Content {
				targets[scalar(mappingValue(entry, "name"))] = true
			}
		}
	}
	providers := mappingValue(root, "proxy-providers")
	if groups := mappingValue(root, "proxy-groups"); groups != nil {
		for index, group := range groups.Content {
			for _, field := range []string{"proxies", "use"} {
				members, err := scalarSequence(mappingValue(group, field))
				if err != nil {
					return fmt.Errorf("proxy-groups[%d].%s: %w", index, field, err)
				}
				for _, name := range members {
					if field == "proxies" && !targets[name] || field == "use" && mappingValue(providers, name) == nil {
						return fmt.Errorf("proxy-groups[%d].%s references a missing resource", index, field)
					}
				}
			}
		}
	}
	ruleProviders := mappingValue(root, "rule-providers")
	subRules := mappingValue(root, "sub-rules")
	validateRules := func(node *yaml.Node, path string) error {
		if node == nil {
			return nil
		}
		if node.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s must be a sequence", path)
		}
		for index, entry := range node.Content {
			if entry.Kind != yaml.ScalarNode || entry.Tag != "!!str" {
				return fmt.Errorf("%s[%d] must be a rule string", path, index)
			}
			parts := splitRuleFields(entry.Value)
			if len(parts) < 2 {
				return fmt.Errorf("%s[%d] has no rule target", path, index)
			}
			last := len(parts) - 1
			if strings.EqualFold(parts[last], "no-resolve") {
				last--
			}
			if last < 1 {
				return fmt.Errorf("%s[%d] has no rule target", path, index)
			}
			if strings.EqualFold(parts[0], "SUB-RULE") {
				if mappingValue(subRules, parts[last]) == nil {
					return fmt.Errorf("%s[%d] references a missing sub-rule", path, index)
				}
			} else if !targets[parts[last]] {
				return fmt.Errorf("%s[%d] references a missing proxy or selector", path, index)
			}
			if strings.EqualFold(parts[0], "RULE-SET") && (len(parts) < 3 || mappingValue(ruleProviders, parts[1]) == nil) {
				return fmt.Errorf("%s[%d] references a missing rule provider", path, index)
			}
		}
		return nil
	}
	if err := validateRules(mappingValue(root, "rules"), "rules"); err != nil {
		return err
	}
	if subRules != nil {
		if subRules.Kind != yaml.MappingNode {
			return fmt.Errorf("sub-rules must be a mapping")
		}
		for index := 1; index < len(subRules.Content); index += 2 {
			if err := validateRules(subRules.Content[index], "sub-rules"); err != nil {
				return err
			}
		}
	}
	return nil
}
