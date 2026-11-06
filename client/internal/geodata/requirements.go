package geodata

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
)

// RequiredKinds inspects configuration fields that consume GEO data. Names,
// URLs, credentials and comments must not become apparent GEO dependencies.
// Remote provider contents remain owned by mihomo and are not fetched here.
func RequiredKinds(configuration []byte, preferences Preferences) ([]Kind, error) {
	parsed, err := document.Parse(configuration)
	if err != nil {
		return nil, fmt.Errorf("inspect GEO data requirements: %w", err)
	}
	root := document.Root(parsed)
	geoIP := SelectedKinds(preferences)[0]
	needed := map[Kind]bool{}
	var inspectionErr error
	var rule func(string, int)
	rule = func(value string, depth int) {
		if inspectionErr != nil {
			return
		}
		if depth > document.DefaultLimits.MaxDepth {
			inspectionErr = fmt.Errorf("inspect GEO data requirements: logical rule nesting exceeds limit")
			return
		}
		parts := splitRequirementRule(value)
		if len(parts) < 2 {
			return
		}
		switch strings.ToUpper(parts[0]) {
		case "GEOIP", "SRC-GEOIP":
			needed[geoIP] = true
		case "GEOSITE":
			needed[KindGeoSite] = true
		case "IP-ASN", "SRC-IP-ASN":
			needed[KindASN] = true
		case "AND", "OR", "NOT":
			for _, child := range splitRequirementRule(unwrapRule(parts[1])) {
				rule(unwrapRule(child), depth+1)
			}
		case "SUB-RULE":
			rule(unwrapRule(parts[1]), depth+1)
		}
	}
	rules := func(node *yaml.Node) {
		node = requirementNode(node)
		if node != nil && node.Kind == yaml.SequenceNode {
			for _, entry := range node.Content {
				entry = requirementNode(entry)
				if entry.Kind == yaml.ScalarNode {
					rule(entry.Value, 0)
				}
			}
		}
	}
	rules(requirementField(root, "rules"))
	if subRules := requirementField(root, "sub-rules"); subRules != nil && subRules.Kind == yaml.MappingNode {
		for i := 1; i < len(subRules.Content); i += 2 {
			rules(subRules.Content[i])
		}
	}
	if providers := requirementField(root, "rule-providers"); providers != nil && providers.Kind == yaml.MappingNode {
		for i := 1; i < len(providers.Content); i += 2 {
			provider := providers.Content[i]
			if requirementValue(provider, "type") == "inline" && requirementValue(provider, "behavior") == "classical" {
				rules(requirementField(provider, "payload"))
			}
		}
	}
	if dns := requirementField(root, "dns"); dns != nil && requirementValue(dns, "enable") == "true" {
		for _, field := range []string{"nameserver-policy", "proxy-server-nameserver-policy"} {
			if field == "proxy-server-nameserver-policy" && !hasRequirementEntries(requirementField(dns, "proxy-server-nameserver")) {
				continue
			}
			if policy := requirementField(dns, field); policy != nil && policy.Kind == yaml.MappingNode {
				for i := 0; i < len(policy.Content); i += 2 {
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(policy.Content[i].Value)), "geosite:") {
						needed[KindGeoSite] = true
					}
				}
			}
		}
		if filters := requirementField(dns, "fake-ip-filter"); filters != nil && filters.Kind == yaml.SequenceNode {
			for _, filter := range filters.Content {
				filter = requirementNode(filter)
				if requirementValue(dns, "fake-ip-filter-mode") == "rule" {
					rule(filter.Value, 0)
				} else if strings.HasPrefix(strings.ToLower(strings.TrimSpace(filter.Value)), "geosite:") {
					needed[KindGeoSite] = true
				}
			}
		}
		if hasRequirementEntries(requirementField(dns, "fallback")) {
			filter := requirementField(dns, "fallback-filter")
			// mihomo enables the CN GeoIP fallback filter by default.
			if requirementValue(filter, "geoip") != "false" {
				needed[geoIP] = true
			}
			if hasRequirementEntries(requirementField(filter, "geosite")) {
				needed[KindGeoSite] = true
			}
		}
	}
	result := []Kind{}
	for _, kind := range SelectedKinds(preferences) {
		if needed[kind] {
			result = append(result, kind)
		}
	}
	return result, inspectionErr
}

func requirementField(node *yaml.Node, key string) *yaml.Node {
	node = requirementNode(node)
	if node != nil && node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return requirementNode(node.Content[i+1])
			}
		}
	}
	return nil
}

func requirementNode(node *yaml.Node) *yaml.Node {
	// document.Parse has already rejected alias cycles and excessive aliases.
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}

func requirementValue(node *yaml.Node, key string) string {
	if value := requirementField(node, key); value != nil {
		return strings.ToLower(strings.TrimSpace(value.Value))
	}
	return ""
}

func hasRequirementEntries(node *yaml.Node) bool {
	return node != nil && (len(node.Content) > 0 || strings.TrimSpace(node.Value) != "")
}

func unwrapRule(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		return value[1 : len(value)-1]
	}
	return value
}

func splitRequirementRule(value string) []string {
	var fields []string
	start, depth := 0, 0
	for i, char := range value {
		switch char {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				fields = append(fields, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	return append(fields, strings.TrimSpace(value[start:]))
}
