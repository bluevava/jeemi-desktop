package resources

import (
	"bytes"
	"net/netip"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
	"gopkg.in/yaml.v3"
)

// RuleSetEntryInput contains one literal match, never caller-supplied YAML.
// The revision guards both the selected definition and the previewed library.
type RuleSetEntryInput struct {
	RuleSetID        string `json:"ruleSetId"`
	Name             string `json:"name"`
	MatchType        string `json:"matchType"`
	Value            string `json:"value"`
	CaseInsensitive  bool   `json:"caseInsensitive"`
	ExpectedRevision int    `json:"expectedRevision"`
}

type RuleSetEntryPreview struct {
	Valid     bool   `json:"valid"`
	Line      string `json:"line"`
	YAMLLine  string `json:"yamlLine"`
	ErrorCode string `json:"errorCode"`
}

type ruleEntryError string

func (e ruleEntryError) Error() string { return "rule entry: " + string(e) }

// PreviewRuleSetEntry uses the same candidate builder as SaveRuleSetEntryValidated.
func (s *Store) PreviewRuleSetEntry(input RuleSetEntryInput) (RuleSetEntryPreview, error) {
	state, err := s.State()
	if err != nil {
		return RuleSetEntryPreview{}, err
	}
	_, preview, err := prepareRuleSetEntry(input, state)
	if code, ok := err.(ruleEntryError); ok {
		return RuleSetEntryPreview{ErrorCode: string(code)}, nil
	}
	return preview, err
}

func (s *Store) SaveRuleSetEntryValidated(input RuleSetEntryInput, validate func(State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	candidate, _, err := prepareRuleSetEntry(input, state)
	if err != nil {
		return State{}, err
	}
	return s.saveRuleSetUnlocked(candidate, state, validate)
}

func prepareRuleSetEntry(input RuleSetEntryInput, state State) (RuleSet, RuleSetEntryPreview, error) {
	fail := func(code string) (RuleSet, RuleSetEntryPreview, error) {
		return RuleSet{}, RuleSetEntryPreview{}, ruleEntryError(code)
	}
	if input.ExpectedRevision != state.Revision {
		return fail("resourcesChanged")
	}
	candidate := RuleSet{Name: strings.TrimSpace(input.Name), SourceType: "inline", Behavior: "classical", Format: "yaml"}
	if input.RuleSetID != "" {
		found := false
		for _, item := range state.RuleSets {
			if item.ID == input.RuleSetID {
				candidate, found = item, true
				break
			}
		}
		if !found {
			return fail("missingRuleSet")
		}
	} else {
		if candidate.Name == "" || utf8.RuneCountInString(candidate.Name) > maxResourceNameLength {
			return fail("invalidName")
		}
		for _, item := range state.RuleSets {
			if item.Name == candidate.Name {
				return fail("duplicateName")
			}
		}
	}
	if candidate.SourceType != "inline" {
		return fail("remoteRuleSet")
	}
	if candidate.Behavior != "classical" &&
		!(candidate.Behavior == "domain" && input.MatchType == "domain") &&
		!(candidate.Behavior == "ipcidr" && input.MatchType == "ip") {
		return fail("incompatibleBehavior")
	}
	line, err := ruleEntryLine(input, candidate.Behavior)
	if err != nil {
		return RuleSet{}, RuleSetEntryPreview{}, err
	}

	var doc yaml.Node
	if candidate.ID == "" {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "payload"},
				{Kind: yaml.SequenceNode, Tag: "!!seq"},
			}},
		}}
	} else {
		// Read only validated payload documents; the AST keeps existing comments,
		// strings and order, including flow-style lists and end-of-document markers.
		if _, _, err := normalizeRuleSetPayloadYAML(candidate.PayloadYAML); err != nil {
			return fail("invalidPayload")
		}
		if err := yaml.Unmarshal([]byte(candidate.PayloadYAML), &doc); err != nil {
			return fail("invalidPayload")
		}
	}
	entry := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.SingleQuotedStyle, Value: line}
	payload := doc.Content[0].Content[1]
	// yaml.v3 omits a flow sequence's trailing comment when changing it to
	// block style. Keep that comment above the sequence instead.
	if payload.Style&yaml.FlowStyle != 0 && payload.LineComment != "" {
		payload.HeadComment = strings.TrimSpace(payload.HeadComment + "\n" + payload.LineComment)
		payload.LineComment = ""
	}
	payload.Style = 0
	payload.Content = append(payload.Content, entry)
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return fail("invalidPayload")
	}
	if err := encoder.Close(); err != nil {
		return fail("invalidPayload")
	}
	candidate.PayloadYAML = output.String()
	source, rules, err := normalizeRuleSetPayloadYAML(candidate.PayloadYAML)
	if err != nil {
		return fail("invalidPayload")
	}
	if err := validateRuleSetPayloadRules(candidate.Behavior, rules); err != nil {
		return fail("invalidPayload")
	}
	candidate.PayloadYAML, candidate.Payload = source, rules
	encodedLine, err := yaml.Marshal(entry)
	if err != nil {
		return fail("invalidPayload")
	}
	return candidate, RuleSetEntryPreview{
		Valid: true, Line: line, YAMLLine: "- " + strings.TrimSuffix(string(encodedLine), "\n"),
	}, nil
}

func ruleEntryLine(input RuleSetEntryInput, behavior string) (string, error) {
	value := strings.TrimSpace(input.Value)
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.IndexFunc(input.Value, unicode.IsControl) >= 0 {
		return "", ruleEntryError("invalidValue")
	}
	switch input.MatchType {
	case "domain":
		value = strings.TrimSuffix(value, ".")
		if strings.ContainsAny(value, "/\\:*,#?@[]() \t") {
			return "", ruleEntryError("invalidDomain")
		}
		ascii, err := idna.Lookup.ToASCII(value)
		if err != nil || len(ascii) > 253 {
			return "", ruleEntryError("invalidDomain")
		}
		value = strings.ToLower(ascii)
		if _, err := netip.ParseAddr(value); err == nil {
			return "", ruleEntryError("invalidDomain")
		}
		for _, label := range strings.Split(value, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", ruleEntryError("invalidDomain")
			}
			for _, char := range label {
				if !(char >= 'a' && char <= 'z') && !(char >= '0' && char <= '9') && char != '-' {
					return "", ruleEntryError("invalidDomain")
				}
			}
		}
		if behavior == "domain" {
			return "+." + value, nil
		}
		return "DOMAIN-SUFFIX," + value, nil
	case "ip":
		prefix, err := netip.ParsePrefix(value)
		if !strings.Contains(value, "/") {
			address, parseErr := netip.ParseAddr(value)
			if parseErr != nil || address.Zone() != "" {
				return "", ruleEntryError("invalidIP")
			}
			address = address.Unmap()
			prefix, err = netip.PrefixFrom(address, address.BitLen()), nil
		}
		if err != nil || !prefix.IsValid() {
			return "", ruleEntryError("invalidIP")
		}
		value = prefix.Masked().String()
		if behavior == "ipcidr" {
			return value, nil
		}
		return "IP-CIDR," + value, nil
	case "processName":
		if strings.ContainsAny(value, ",/\\") {
			return "", ruleEntryError("invalidProcessName")
		}
		return "PROCESS-NAME," + value, nil
	case "processPath":
		// The form takes an unescaped literal path/fragment. Quote once, including
		// dots, parentheses and Windows backslashes. Slash needs no RE2 escaping.
		pattern := regexp.QuoteMeta(value)
		// A comma is a mihomo rule separator even when YAML-quoted.
		pattern = strings.ReplaceAll(pattern, ",", `\x2c`)
		if input.CaseInsensitive {
			pattern = "(?i)" + pattern
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return "", ruleEntryError("invalidValue")
		}
		return "PROCESS-PATH-REGEX," + pattern, nil
	default:
		return "", ruleEntryError("invalidMatchType")
	}
}

var _ error = ruleEntryError("")
