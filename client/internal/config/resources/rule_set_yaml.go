package resources

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxPayloadYAMLBytes = 2 << 20

var classicalRuleTypes = map[string]struct{}{
	"DOMAIN": {}, "DOMAIN-SUFFIX": {}, "DOMAIN-KEYWORD": {}, "DOMAIN-WILDCARD": {},
	"DOMAIN-REGEX": {}, "GEOSITE": {}, "IP-CIDR": {}, "IP-CIDR6": {}, "IP-SUFFIX": {},
	"IP-ASN": {}, "GEOIP": {}, "SRC-GEOIP": {}, "SRC-IP-ASN": {}, "SRC-IP-CIDR": {},
	"SRC-IP-SUFFIX": {}, "DST-PORT": {}, "SRC-PORT": {}, "IN-PORT": {}, "IN-TYPE": {},
	"IN-USER": {}, "IN-NAME": {}, "REMATCH-NAME": {}, "PROCESS-PATH": {},
	"PROCESS-PATH-WILDCARD": {}, "PROCESS-PATH-REGEX": {}, "PROCESS-NAME": {},
	"PROCESS-NAME-WILDCARD": {}, "PROCESS-NAME-REGEX": {}, "UID": {}, "NETWORK": {},
	"DSCP": {}, "AND": {}, "OR": {}, "NOT": {},
}

// normalizeRuleSetPayloadYAML validates the narrow document accepted by a
// mihomo inline rule provider. Keeping the original YAML text preserves user
// comments, while the returned payload is the normalized representation used
// by the configuration composer.
func normalizeRuleSetPayloadYAML(source string) (string, []string, error) {
	source = normalizeYAMLText(source)
	if strings.TrimSpace(source) == "" {
		return "", nil, fmt.Errorf("inline rule set YAML is required and must contain payload")
	}
	if len(source) > maxPayloadYAMLBytes {
		return "", nil, fmt.Errorf("inline rule set YAML exceeds the %d byte limit", maxPayloadYAMLBytes)
	}

	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(source))
	if err := decoder.Decode(&document); err != nil {
		return "", nil, fmt.Errorf("parse inline rule set YAML: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != nil {
		if !errors.Is(err, io.EOF) {
			return "", nil, fmt.Errorf("parse trailing inline rule set YAML: %w", err)
		}
	} else {
		return "", nil, fmt.Errorf("inline rule set YAML must contain exactly one document")
	}
	if len(document.Content) != 1 {
		return "", nil, fmt.Errorf("inline rule set YAML must contain exactly one document")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode || root.Tag != "!!map" || root.Anchor != "" || root.Alias != nil || len(root.Content) != 2 {
		return "", nil, fmt.Errorf("inline rule set YAML must be an object containing only payload")
	}
	key, payload := root.Content[0], root.Content[1]
	if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value != "payload" || key.Anchor != "" || key.Alias != nil {
		return "", nil, fmt.Errorf("inline rule set YAML only allows the payload field")
	}
	if payload.Kind != yaml.SequenceNode || payload.Tag != "!!seq" || payload.Anchor != "" || payload.Alias != nil {
		return "", nil, fmt.Errorf("inline rule set payload must be a list without anchors or aliases")
	}
	if len(payload.Content) == 0 || len(payload.Content) > maxPayloadRules {
		return "", nil, fmt.Errorf("inline rule set payload must contain 1-%d rules", maxPayloadRules)
	}

	rules := make([]string, 0, len(payload.Content))
	for _, item := range payload.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Anchor != "" || item.Alias != nil {
			return "", nil, fmt.Errorf("inline rule set payload line %d must be plain text without anchors or aliases", item.Line)
		}
		rule := strings.TrimSpace(item.Value)
		if rule == "" || strings.ContainsAny(rule, "\r\n") {
			return "", nil, fmt.Errorf("inline rule set payload line %d is empty or multiline", item.Line)
		}
		rules = append(rules, rule)
	}
	return source, rules, nil
}

func validateRuleSetPayloadRules(behavior string, payload []string) error {
	for index, rule := range payload {
		switch behavior {
		case "domain":
			if strings.Contains(rule, ",") {
				return fmt.Errorf("domain payload item %d must contain only a domain pattern", index+1)
			}
		case "ipcidr":
			if _, err := netip.ParsePrefix(rule); err != nil {
				return fmt.Errorf("ipcidr payload item %d is not a valid CIDR prefix", index+1)
			}
		case "classical":
			typeText, value, found := strings.Cut(rule, ",")
			typeName := strings.ToUpper(strings.TrimSpace(typeText))
			if !found || strings.TrimSpace(value) == "" {
				return fmt.Errorf("classical payload item %d is missing its match value", index+1)
			}
			if _, allowed := classicalRuleTypes[typeName]; !allowed {
				return fmt.Errorf("classical payload item %d uses unsupported rule type %q", index+1, typeName)
			}
		default:
			return fmt.Errorf("rule set behavior is unsupported")
		}
	}
	return nil
}

func encodeRuleSetPayloadYAML(payload []string) (string, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "payload"},
		{Kind: yaml.SequenceNode, Tag: "!!seq"},
	}}
	for _, value := range payload {
		root.Content[1].Content = append(root.Content[1].Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: strings.TrimSpace(value),
		})
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}); err != nil {
		return "", fmt.Errorf("encode inline rule set YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close inline rule set YAML encoder: %w", err)
	}
	return output.String(), nil
}

func normalizeYAMLText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.TrimRight(value, " \t\n")
	if value == "" {
		return ""
	}
	return value + "\n"
}
