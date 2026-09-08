package fallbackoverride

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/config/document"
)

func TestFallbackPreservesBoundarySourceAndSubRules(t *testing.T) {
	input := []byte(`proxy-groups:
  - {name: Original, type: select, proxies: [DIRECT]}
  - {name: '🌐 Hidden', type: fallback, hidden: true, proxies: [DIRECT]}
rules:
  - DOMAIN,first.example,DIRECT
  - FINAL,Original # terminal comment
  - DOMAIN,unreachable.example,REJECT
  - MATCH,REJECT
sub-rules:
  child: ["MATCH,Original"]
`)
	for _, test := range []struct {
		name      string
		selection Selection
		target    string
		reset     bool
	}{
		{"none", Selection{Mode: ModeNone}, "Original", false},
		{"direct", Selection{Mode: ModeDirect}, "DIRECT", false},
		{"hidden selector", Selection{Mode: ModeSelector, Selector: "🌐 Hidden"}, "🌐 Hidden", false},
		{"exact name required", Selection{Mode: ModeSelector, Selector: "original"}, "Original", true},
		{"missing", Selection{Mode: ModeSelector, Selector: "Missing"}, "Original", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := append([]byte{}, input...)
			output, state, err := Apply(input, test.selection)
			if err != nil {
				t.Fatal(err)
			}
			if state.OriginalTarget != "Original" || state.Reset != test.reset || !reflect.DeepEqual(state.Selectors, []string{"Original", "🌐 Hidden"}) {
				t.Fatalf("incorrect source projection: %+v", state)
			}
			if !bytes.Equal(input, before) {
				t.Fatal("input buffer changed")
			}
			if test.selection.Mode == ModeNone || test.reset {
				if !bytes.Equal(output, input) || state.Selection.Mode != ModeNone {
					t.Fatal("no override must preserve source bytes")
				}
			}
			parsed, err := document.Parse(output)
			if err != nil {
				t.Fatal(err)
			}
			rules, _, _ := document.Find(document.Root(parsed), "/rules")
			if len(rules.Content) != 4 || rules.Content[1].Value != "FINAL,"+test.target || rules.Content[2].Value != "DOMAIN,unreachable.example,REJECT" || rules.Content[3].Value != "MATCH,REJECT" {
				t.Fatalf("terminal boundary changed: %s", output)
			}
			if rules.Content[1].LineComment != "# terminal comment" {
				t.Fatal("terminal comment was lost")
			}
			subRules, _, _ := document.Find(document.Root(parsed), "/sub-rules/child")
			if len(subRules.Content) != 1 || subRules.Content[0].Value != "MATCH,Original" {
				t.Fatal("sub-rules changed")
			}
		})
	}
}

func TestFallbackMissingTerminalAndInvalidInput(t *testing.T) {
	for _, source := range []string{"{}\n", "rules: []\n", "rules: [\"DOMAIN,example.org,DIRECT\"]\n"} {
		plain, state, err := Apply([]byte(source), Selection{})
		if err != nil || string(plain) != source || state.OriginalTarget != "" {
			t.Fatalf("no override invented a terminal: %s, %v", plain, err)
		}
		injected, _, err := Apply([]byte(source), Selection{Mode: ModeDirect})
		if err != nil || !strings.Contains(string(injected), "MATCH,DIRECT") {
			t.Fatalf("explicit override failed to append terminal: %s, %v", injected, err)
		}
	}
	for _, source := range []string{"rules: {}", "rules: [true]", "proxy-groups: {}", "proxy-groups: [{name: X, type: select}, {name: X, type: select}]"} {
		if _, _, err := Apply([]byte(source), Selection{Mode: ModeSelector, Selector: "Gone"}); err == nil {
			t.Fatalf("invalid composition accepted: %s", source)
		}
	}
	for _, selection := range []Selection{{Mode: "invalid"}, {Mode: ModeDirect, Selector: "X"}, {Mode: ModeSelector}, {Mode: ModeSelector, Selector: "A,REJECT"}, {Mode: ModeSelector, Selector: "A\nB"}} {
		if _, err := Normalize(selection); err == nil {
			t.Fatalf("invalid preference accepted: %+v", selection)
		}
	}
}
