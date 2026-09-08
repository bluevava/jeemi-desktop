// Package fallbackoverride owns the per-subscription terminal rule injection.
// It never persists preferences or changes subscription/local source files.
package fallbackoverride

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"jeemi/internal/config/document"
	configinspect "jeemi/internal/config/inspect"

	"gopkg.in/yaml.v3"
)

const (
	ModeNone     = "none"
	ModeDirect   = "direct"
	ModeSelector = "selector"
)

type Selection struct {
	Mode     string `json:"mode"`
	Selector string `json:"selector"`
}

type State struct {
	Selection      Selection `json:"selection"`
	OriginalTarget string    `json:"originalTarget"`
	Selectors      []string  `json:"selectors"`
	Reset          bool      `json:"reset"`
}

func Normalize(input Selection) (Selection, error) {
	if input.Mode == "" {
		input.Mode = ModeNone
	}
	switch input.Mode {
	case ModeNone, ModeDirect:
		if input.Selector != "" {
			return Selection{}, fmt.Errorf("fallback selector must be empty for this mode")
		}
	case ModeSelector:
		if input.Selector == "" || strings.TrimSpace(input.Selector) != input.Selector ||
			utf8.RuneCountInString(input.Selector) > 256 || strings.ContainsRune(input.Selector, ',') ||
			strings.ContainsFunc(input.Selector, unicode.IsControl) {
			return Selection{}, fmt.Errorf("fallback selector name is invalid")
		}
	default:
		return Selection{}, fmt.Errorf("fallback mode is unsupported")
	}
	return input, nil
}

// Apply inspects the complete local/script result, including hidden selectors.
// A missing selector resolves to none; callers may persist that reset only
// after the entire composition succeeds, never during a preview or script test.
func Apply(contents []byte, input Selection) ([]byte, State, error) {
	selection, err := Normalize(input)
	if err != nil {
		return nil, State{}, err
	}
	inspected, err := configinspect.Configuration(contents)
	if err != nil {
		return nil, State{}, err
	}
	state := State{Selection: selection, Selectors: []string{}}
	found := false
	seen := make(map[string]struct{}, len(inspected.Selectors))
	for _, selector := range inspected.Selectors {
		if _, duplicate := seen[selector.Name]; duplicate {
			return nil, State{}, fmt.Errorf("configuration contains duplicate selector names")
		}
		seen[selector.Name] = struct{}{}
		state.Selectors = append(state.Selectors, selector.Name)
		found = found || selector.Name == selection.Selector
	}
	if selection.Mode == ModeSelector && !found {
		state.Selection = Selection{Mode: ModeNone}
		state.Reset = true
	}
	parsed, err := document.Parse(contents)
	if err != nil {
		return nil, State{}, err
	}
	root := document.Root(parsed)
	rules, exists, err := document.Find(root, "/rules")
	if err != nil {
		return nil, State{}, err
	}
	if !exists {
		rules = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	if rules.Kind != yaml.SequenceNode {
		return nil, State{}, fmt.Errorf("configuration rules must be a sequence")
	}
	var terminal *yaml.Node
	var parts []string
	for _, rule := range rules.Content {
		if rule.Kind != yaml.ScalarNode || rule.Tag != "!!str" {
			return nil, State{}, fmt.Errorf("configuration rules must contain strings")
		}
		candidate := strings.Split(rule.Value, ",")
		kind := strings.ToUpper(strings.TrimSpace(candidate[0]))
		if terminal == nil && (kind == "MATCH" || kind == "FINAL") {
			terminal, parts = rule, candidate
			if len(parts) > 1 {
				state.OriginalTarget = strings.TrimSpace(parts[1])
			}
		}
	}
	if state.Selection.Mode == ModeNone {
		return contents, state, nil
	}
	target := "DIRECT"
	if state.Selection.Mode == ModeSelector {
		target = state.Selection.Selector
	}
	if terminal == nil {
		rules.Content = append(rules.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "MATCH," + target})
	} else {
		// Replace in place: moving MATCH past trailing rules would make formerly
		// unreachable rules active. Keep its spelling, flags and YAML comments.
		if len(parts) < 2 {
			parts = append(parts, target)
		} else {
			parts[1] = target
		}
		terminal.Value = strings.Join(parts, ",")
	}
	if err := document.SetMappingPath(root, "/rules", rules); err != nil {
		return nil, State{}, err
	}
	output, err := document.Encode(parsed)
	return output, state, err
}
