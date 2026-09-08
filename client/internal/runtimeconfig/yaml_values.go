package runtimeconfig

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

const (
	YAMLFieldDNSNameserver                  = "dns.nameserver"
	YAMLFieldDNSFakeIPFilter                = "dns.fake-ip-filter"
	YAMLFieldDNSProxyServerNameserver       = "dns.proxy-server-nameserver"
	YAMLFieldDNSNameserverPolicy            = "dns.nameserver-policy"
	YAMLFieldDNSProxyServerNameserverPolicy = "dns.proxy-server-nameserver-policy"
	YAMLFieldHosts                          = "hosts"
	YAMLFieldLANBypassRules                 = "rules.lan-bypass"
	YAMLFieldTUNRouteExcludeAddress         = "tun.route-exclude-address"
	maxLANBypassRuleCount                   = 4096
)

var yamlLinePattern = regexp.MustCompile(`(?i)line\s+([0-9]+)(?::([0-9]+))?`)

// YAMLFragmentInput is the narrow validation request used by the Home YAML
// editors. Field is deliberately an allow-listed value; arbitrary paths are
// never accepted from React.
type YAMLFragmentInput struct {
	Field    string `json:"field"`
	Contents string `json:"contents"`
}

type YAMLValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type YAMLFragmentValidation struct {
	Field          string               `json:"field"`
	Valid          bool                 `json:"valid"`
	NormalizedYAML string               `json:"normalizedYaml"`
	Values         []string             `json:"values"`
	Issue          *YAMLValidationIssue `json:"issue"`
}

// ValidateYAMLFragment parses one of the fixed Home editor fragments without
// saving settings or starting configuration reconciliation. Syntax and shape
// errors are returned as data so the renderer can keep the draft and return
// focus to the exact editor.
func ValidateYAMLFragment(input YAMLFragmentInput) YAMLFragmentValidation {
	field := strings.TrimSpace(input.Field)
	result := YAMLFragmentValidation{Field: field, Values: []string{}}
	var (
		normalized string
		values     []string
		err        error
	)
	switch field {
	case YAMLFieldDNSNameserver:
		values, normalized, err = parseStringSequenceYAML(input.Contents, "DNS nameserver", 1, maxResolverCount)
	case YAMLFieldDNSFakeIPFilter:
		values, normalized, err = parseStringSequenceYAML(input.Contents, "DNS fake IP filter", 0, maxFakeIPFilterCount)
	case YAMLFieldDNSProxyServerNameserver:
		values, normalized, err = parseStringSequenceYAML(input.Contents, "DNS proxy server nameserver", 0, maxResolverCount)
	case YAMLFieldDNSNameserverPolicy:
		normalized, err = parseStringMappingYAML(input.Contents, "DNS nameserver policy")
	case YAMLFieldDNSProxyServerNameserverPolicy:
		normalized, err = parseStringMappingYAML(input.Contents, "DNS proxy server nameserver policy")
	case YAMLFieldHosts:
		normalized, err = parseStringMappingYAML(input.Contents, "hosts")
	case YAMLFieldLANBypassRules:
		values, normalized, err = parseStringSequenceYAML(input.Contents, "LAN bypass rules", 0, maxLANBypassRuleCount)
		if err == nil {
			err = ValidateLANBypassRules(values)
		}
	case YAMLFieldTUNRouteExcludeAddress:
		values, normalized, err = parseStringSequenceYAML(input.Contents, "TUN route exclude addresses", 0, maxTUNRouteExcludeCount)
		if err == nil {
			err = ValidateTUNRouteExcludeAddresses(values)
		}
	default:
		err = fmt.Errorf("unsupported runtime YAML field %q", field)
	}
	if err != nil {
		result.Issue = issueFromYAMLError(err)
		return result
	}
	result.Valid = true
	result.NormalizedYAML = normalized
	result.Values = values
	return result
}

func ValidateResolverPolicyYAML(contents string) error {
	return validateStringMapping(contents, "resolver policy")
}

func ValidateHostsYAML(contents string) error {
	return validateStringMapping(contents, "hosts")
}

// ParseStringMapping converts a validated Home YAML mapping into a detached
// node for final configuration injection. Blank input intentionally means an
// empty mapping, which is useful with append mode and optional DNS policies.
func ParseStringMapping(contents string) (*yaml.Node, error) {
	if strings.TrimSpace(contents) == "" {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	value, err := document.ParseValue(contents)
	if err != nil {
		return nil, err
	}
	if err := validateStringMappingNode(value, "mapping"); err != nil {
		return nil, err
	}
	return value, nil
}

func parseStringMappingYAML(contents, label string) (string, error) {
	if strings.TrimSpace(contents) == "" {
		return "", nil
	}
	if err := validateStringMapping(contents, label); err != nil {
		return "", err
	}
	value, err := ParseStringMapping(contents)
	if err != nil {
		return "", err
	}
	if len(value.Content) == 0 {
		return "", nil
	}
	return document.EncodeValue(value)
}

func parseStringSequenceYAML(contents, label string, minimum, maximum int) ([]string, string, error) {
	if len(contents) > maxMappingYAMLBytes {
		return nil, "", fmt.Errorf("%s exceeds the %d byte limit", label, maxMappingYAMLBytes)
	}
	trimmed := strings.TrimSpace(contents)
	if trimmed == "" {
		if minimum > 0 {
			return nil, "", fmt.Errorf("%s must contain between %d and %d entries", label, minimum, maximum)
		}
		return []string{}, "", nil
	}
	value, err := document.ParseValue(trimmed)
	if err != nil {
		return nil, "", err
	}
	if value.Kind != yaml.SequenceNode {
		return nil, "", fmt.Errorf("line %d: %s must be a YAML sequence", value.Line, label)
	}
	if hasAnchorOrAlias(value) {
		return nil, "", fmt.Errorf("line %d: %s cannot contain YAML anchors or aliases", value.Line, label)
	}
	if len(value.Content) < minimum || len(value.Content) > maximum {
		return nil, "", fmt.Errorf("%s must contain between %d and %d entries", label, minimum, maximum)
	}
	values := make([]string, 0, len(value.Content))
	for _, item := range value.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return nil, "", fmt.Errorf("line %d: %s entries must be strings", item.Line, label)
		}
		entry := strings.TrimSpace(item.Value)
		if entry == "" || len(entry) > maxResolverLength || strings.ContainsAny(entry, "\r\n\x00") {
			return nil, "", fmt.Errorf("line %d: %s contains an invalid entry", item.Line, label)
		}
		values = append(values, entry)
	}
	values = normalizeStringList(values)
	if len(values) < minimum {
		return nil, "", fmt.Errorf("%s must contain between %d and %d unique entries", label, minimum, maximum)
	}
	if len(values) == 0 {
		return values, "", nil
	}
	normalized, err := encodeStringSequenceYAML(values)
	if err != nil {
		return nil, "", err
	}
	return values, normalized, nil
}

// normalizeMappingEditorYAML keeps persisted empty mappings visually empty in
// multiline editors while preserving invalid dormant text for later diagnosis.
func normalizeMappingEditorYAML(contents string) string {
	trimmed := strings.TrimSpace(contents)
	if trimmed == "" {
		return ""
	}
	value, err := document.ParseValue(trimmed)
	if err == nil && value.Kind == yaml.MappingNode && len(value.Content) == 0 {
		return ""
	}
	return trimmed
}

func encodeStringSequenceYAML(values []string) (string, error) {
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	return document.EncodeValue(sequence)
}

// ValidateLANBypassRules constrains the editable prefix to the same safe
// subset as an inline classical DIRECT rule set. An empty list is valid and
// means Jeemi does not prepend any rules.
func ValidateLANBypassRules(rules []string) error {
	if rules == nil || len(rules) > maxLANBypassRuleCount {
		return fmt.Errorf("must contain between 0 and %d entries", maxLANBypassRuleCount)
	}
	for index, rule := range rules {
		parts := strings.Split(rule, ",")
		for partIndex := range parts {
			parts[partIndex] = strings.TrimSpace(parts[partIndex])
		}
		line := index + 1
		if len(parts) < 3 {
			return fmt.Errorf("line %d: rule must contain a type, value, and DIRECT target", line)
		}
		if parts[2] != "DIRECT" {
			return fmt.Errorf("line %d: rule target must be DIRECT", line)
		}
		switch parts[0] {
		case "DOMAIN", "DOMAIN-SUFFIX":
			if len(parts) != 3 || parts[1] == "" {
				return fmt.Errorf("line %d: %s rule must contain exactly a non-empty value and DIRECT target", line, parts[0])
			}
		case "IP-CIDR", "IP-CIDR6":
			if len(parts) != 4 || parts[3] != "no-resolve" {
				return fmt.Errorf("line %d: %s rule must end with DIRECT,no-resolve", line, parts[0])
			}
			prefix, err := netip.ParsePrefix(parts[1])
			if err != nil {
				return fmt.Errorf("line %d: %s contains an invalid CIDR", line, parts[0])
			}
			if parts[0] == "IP-CIDR" && !prefix.Addr().Is4() {
				return fmt.Errorf("line %d: IP-CIDR requires an IPv4 prefix", line)
			}
			if parts[0] == "IP-CIDR6" && !prefix.Addr().Is6() {
				return fmt.Errorf("line %d: IP-CIDR6 requires an IPv6 prefix", line)
			}
		default:
			return fmt.Errorf("line %d: rule type %q is not supported", line, parts[0])
		}
	}
	return nil
}

// ValidateTUNRouteExcludeAddresses accepts only explicit IPv4 or IPv6 CIDR
// prefixes. An empty list is valid and means that Home clears any route
// exclusions inherited from a subscription or local configuration.
func ValidateTUNRouteExcludeAddresses(addresses []string) error {
	if addresses == nil || len(addresses) > maxTUNRouteExcludeCount {
		return fmt.Errorf("must contain between 0 and %d entries", maxTUNRouteExcludeCount)
	}
	for index, address := range addresses {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" || len(trimmed) > maxResolverLength || strings.ContainsAny(trimmed, "\r\n\x00") {
			return fmt.Errorf("line %d: route exclusion contains an invalid entry", index+1)
		}
		if _, err := netip.ParsePrefix(trimmed); err != nil {
			return fmt.Errorf("line %d: route exclusion must be an IPv4 or IPv6 CIDR", index+1)
		}
	}
	return nil
}

func issueFromYAMLError(err error) *YAMLValidationIssue {
	issue := &YAMLValidationIssue{Code: "runtime_yaml_invalid", Message: err.Error()}
	matches := yamlLinePattern.FindStringSubmatch(issue.Message)
	if len(matches) > 1 {
		issue.Line, _ = strconv.Atoi(matches[1])
	}
	if len(matches) > 2 {
		issue.Column, _ = strconv.Atoi(matches[2])
	}
	return issue
}

func validateStringMapping(contents, label string) error {
	if len(contents) > maxMappingYAMLBytes {
		return fmt.Errorf("%s exceeds the %d byte limit", label, maxMappingYAMLBytes)
	}
	if strings.TrimSpace(contents) == "" {
		return nil
	}
	value, err := document.ParseValue(contents)
	if err != nil {
		return err
	}
	return validateStringMappingNode(value, label)
}

func validateStringMappingNode(value *yaml.Node, label string) error {
	if value == nil || value.Kind != yaml.MappingNode {
		if value != nil && value.Line > 0 {
			return fmt.Errorf("line %d:%d: %s must be a YAML mapping", value.Line, value.Column, label)
		}
		return fmt.Errorf("%s must be a YAML mapping", label)
	}
	if hasAnchorOrAlias(value) {
		return fmt.Errorf("%s cannot contain YAML anchors or aliases", label)
	}
	if len(value.Content)/2 > maxResolverCount {
		return fmt.Errorf("%s contains too many entries", label)
	}
	for index := 0; index < len(value.Content); index += 2 {
		key := value.Content[index]
		item := value.Content[index+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || strings.TrimSpace(key.Value) == "" || len(key.Value) > maxResolverLength {
			return fmt.Errorf("line %d:%d: %s contains an invalid key", key.Line, key.Column, label)
		}
		switch item.Kind {
		case yaml.ScalarNode:
			if item.Tag != "!!str" || strings.TrimSpace(item.Value) == "" || len(item.Value) > maxResolverLength {
				return fmt.Errorf("line %d:%d: %s contains an invalid value for %q", item.Line, item.Column, label, key.Value)
			}
		case yaml.SequenceNode:
			if len(item.Content) == 0 || len(item.Content) > maxResolverCount {
				return fmt.Errorf("line %d:%d: %s contains an invalid list for %q", item.Line, item.Column, label, key.Value)
			}
			for _, member := range item.Content {
				if member.Kind != yaml.ScalarNode || member.Tag != "!!str" || strings.TrimSpace(member.Value) == "" || len(member.Value) > maxResolverLength {
					return fmt.Errorf("line %d:%d: %s contains an invalid list value for %q", member.Line, member.Column, label, key.Value)
				}
			}
		default:
			return fmt.Errorf("line %d:%d: %s values must be strings or string lists", item.Line, item.Column, label)
		}
	}
	return nil
}

func hasAnchorOrAlias(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return true
	}
	for _, child := range node.Content {
		if hasAnchorOrAlias(child) {
			return true
		}
	}
	return false
}
