package resources

import (
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRuleEntryMatchesAndYAMLRoundTrip(t *testing.T) {
	tests := []struct {
		name, kind, value, line string
		ignoreCase              bool
	}{
		{"domain", "domain", " Anthropic.COM. ", "DOMAIN-SUFFIX,anthropic.com", false},
		{"unicode domain", "domain", "例子.测试", "DOMAIN-SUFFIX,xn--fsqu00a.xn--0zwm56d", false},
		{"ipv4 host", "ip", "20.203.144.245", "IP-CIDR,20.203.144.245/32", false},
		{"ipv6 host", "ip", "2001:db8::1", "IP-CIDR,2001:db8::1/128", false},
		{"ipv4 mapped host", "ip", "::ffff:20.203.144.245", "IP-CIDR,20.203.144.245/32", false},
		{"ipv4 subnet", "ip", "20.203.144.245/24", "IP-CIDR,20.203.144.0/24", false},
		{"ipv6 subnet", "ip", "2001:db8::1234/64", "IP-CIDR,2001:db8::/64", false},
		{"process", "processName", "WeChat.exe", "PROCESS-NAME,WeChat.exe", false},
		{"yaml punctuation", "processName", "Bob's App: #1.exe", "PROCESS-NAME,Bob's App: #1.exe", false},
		{"windows fragment", "processPath", `\Microsoft\Edge`, `PROCESS-PATH-REGEX,(?i)\\Microsoft\\Edge`, true},
		{"windows full path", "processPath", `C:\Program Files (x86)\Edge\msedge.exe`, `PROCESS-PATH-REGEX,(?i)C:\\Program Files \(x86\)\\Edge\\msedge\.exe`, true},
		{"unc path", "processPath", `\\server\share\App.exe`, `PROCESS-PATH-REGEX,\\\\server\\share\\App\.exe`, false},
		{"unix path", "processPath", "/Applications/Example.app/", `PROCESS-PATH-REGEX,/Applications/Example\.app/`, false},
		{"comma in path", "processPath", "/opt/a,b/app", `PROCESS-PATH-REGEX,/opt/a\x2cb/app`, false},
		{"regex syntax stays literal", "processPath", "/opt/[app]+/.*", `PROCESS-PATH-REGEX,/opt/\[app\]\+/\.\*`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := RuleSetEntryInput{Name: "未命名1", MatchType: tt.kind, Value: tt.value, CaseInsensitive: tt.ignoreCase}
			candidate, preview, err := prepareRuleSetEntry(input, State{})
			if err != nil || !preview.Valid || preview.Line != tt.line {
				t.Fatalf("preview = %+v, err = %v; want %q", preview, err, tt.line)
			}
			_, rules, err := normalizeRuleSetPayloadYAML(candidate.PayloadYAML)
			if err != nil || !reflect.DeepEqual(rules, []string{tt.line}) {
				t.Fatalf("payload did not round trip: %q, %v", candidate.PayloadYAML, err)
			}
			var shown []string
			if err := yaml.Unmarshal([]byte(preview.YAMLLine), &shown); err != nil || !reflect.DeepEqual(shown, rules) {
				t.Fatalf("displayed YAML differs from saved rule: %q, %v", preview.YAMLLine, err)
			}
			if strings.Contains(preview.YAMLLine, "\n") {
				t.Fatal("preview is not a single YAML line")
			}
			if tt.kind == "processPath" {
				pattern := strings.TrimPrefix(preview.Line, "PROCESS-PATH-REGEX,")
				matcher, err := regexp.Compile(pattern)
				if err != nil || !matcher.MatchString(tt.value) {
					t.Fatalf("escaped regex does not match the original path: %q, %v", pattern, err)
				}
				if tt.ignoreCase && !matcher.MatchString(strings.ToLower(tt.value)) {
					t.Fatal("ignore-case regex was lost")
				}
			}
		})
	}
}

func TestRuleEntryRejectsInvalidValues(t *testing.T) {
	tests := []struct{ kind, value, code string }{
		{"domain", "", "invalidValue"},
		{"domain", "example.com\npayload: [bad]", "invalidValue"},
		{"processPath", "/app/\x00name", "invalidValue"},
		{"processPath", strings.Repeat("a", 4097), "invalidValue"},
		{"domain", "https://anthropic.com/path", "invalidDomain"},
		{"domain", "*.example.com", "invalidDomain"},
		{"domain", "1.2.3.4", "invalidDomain"},
		{"domain", "-example.com", "invalidDomain"},
		{"domain", "example..com", "invalidDomain"},
		{"domain", strings.Repeat("a", 64) + ".com", "invalidDomain"},
		{"ip", "256.1.1.1", "invalidIP"},
		{"ip", "1.2.3.4/33", "invalidIP"},
		{"ip", "2001:db8::1/129", "invalidIP"},
		{"ip", "fe80::1%eth0", "invalidIP"},
		{"ip", "example.com", "invalidIP"},
		{"processName", `C:\Apps\WeChat.exe`, "invalidProcessName"},
		{"processName", "a,REJECT", "invalidProcessName"},
		{"unknown", "example", "invalidMatchType"},
	}
	for _, tt := range tests {
		_, err := ruleEntryLine(RuleSetEntryInput{MatchType: tt.kind, Value: tt.value}, "classical")
		if err != ruleEntryError(tt.code) {
			t.Errorf("%s %q: got %v, want %s", tt.kind, tt.value, err, tt.code)
		}
	}
}

func TestRuleEntryPreviewDoesNotCreateDraftAndSaveCreatesClassical(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	input := RuleSetEntryInput{Name: "未命名1", MatchType: "domain", Value: "anthropic.com"}
	preview, err := store.PreviewRuleSetEntry(input)
	if err != nil || !preview.Valid {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	if _, err := os.Stat(store.file); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview created a persistent rule set")
	}
	state, err := store.SaveRuleSetEntryValidated(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 1 || len(state.RuleSets) != 1 {
		t.Fatalf("state: %+v", state)
	}
	rules := state.RuleSets[0]
	if rules.Name != input.Name || rules.Behavior != "classical" || rules.SourceType != "inline" ||
		rules.Format != "yaml" || rules.Payload[0] != preview.Line {
		t.Fatalf("new rule set differs from preview: %+v", rules)
	}
	input.ExpectedRevision = state.Revision
	duplicate, err := store.PreviewRuleSetEntry(input)
	if err != nil || duplicate.Valid || duplicate.ErrorCode != "duplicateName" {
		t.Fatalf("duplicate draft name: %+v, %v", duplicate, err)
	}
}

func TestRuleEntryAppendsAfterExistingYAMLAndPreservesComments(t *testing.T) {
	for _, source := range []string{
		"# document\npayload:\n  # first rule\n  - DOMAIN,example.com # inline comment\n  - 'PROCESS-NAME,old.exe'\n# footer\n",
		"# document\npayload: ['DOMAIN,example.com', 'PROCESS-NAME,old.exe'] # inline comment\n...\n# footer\n",
	} {
		store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		state, err := store.SaveRuleSet(RuleSet{Name: "Existing", Description: "Keep description", SourceType: "inline", Behavior: "classical", PayloadYAML: source})
		if err != nil {
			t.Fatal(err)
		}
		before := state.RuleSets[0]
		input := RuleSetEntryInput{RuleSetID: before.ID, ExpectedRevision: state.Revision, MatchType: "processPath", Value: `C:\Bob's App: #1\app.exe`}
		preview, err := store.PreviewRuleSetEntry(input)
		if err != nil || !preview.Valid {
			t.Fatalf("preview: %+v, %v", preview, err)
		}
		state, err = store.SaveRuleSetEntryValidated(input, nil)
		if err != nil {
			t.Fatal(err)
		}
		after := state.RuleSets[0]
		if !reflect.DeepEqual(after.Payload, append(before.Payload, preview.Line)) ||
			after.ID != before.ID || after.CreatedAt != before.CreatedAt ||
			after.Name != before.Name || after.Description != before.Description {
			t.Fatalf("append altered the existing definition: %+v", after)
		}
		for _, comment := range []string{"# document", "# inline comment", "# footer"} {
			if !strings.Contains(after.PayloadYAML, comment) {
				t.Fatalf("lost comment %q: %s", comment, after.PayloadYAML)
			}
		}
		_, payload, err := normalizeRuleSetPayloadYAML(after.PayloadYAML)
		if err != nil || !reflect.DeepEqual(payload, after.Payload) {
			t.Fatalf("appended punctuation became YAML syntax: %v", err)
		}
	}
}

func TestRuleEntryRespectsExistingBehaviorAndSource(t *testing.T) {
	for _, tt := range []struct{ behavior, kind, value, expected string }{
		{"domain", "domain", "anthropic.com", "+.anthropic.com"},
		{"ipcidr", "ip", "2001:db8::1", "2001:db8::1/128"},
	} {
		store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		state, err := store.SaveRuleSet(RuleSet{Name: "Existing", SourceType: "inline", Behavior: tt.behavior, NoResolve: tt.behavior == "ipcidr", PayloadYAML: "payload:\n  - '" + tt.expected + "'\n"})
		if err != nil {
			t.Fatal(err)
		}
		input := RuleSetEntryInput{RuleSetID: state.RuleSets[0].ID, ExpectedRevision: state.Revision, MatchType: tt.kind, Value: tt.value}
		preview, err := store.PreviewRuleSetEntry(input)
		if err != nil || preview.Line != tt.expected {
			t.Fatalf("preview: %+v, %v", preview, err)
		}
		saved, err := store.SaveRuleSetEntryValidated(input, nil)
		if err != nil || saved.RuleSets[0].NoResolve != state.RuleSets[0].NoResolve {
			t.Fatalf("behavior settings lost: %+v, %v", saved, err)
		}
		input.MatchType, input.Value, input.ExpectedRevision = "processName", "WeChat.exe", saved.Revision
		preview, err = store.PreviewRuleSetEntry(input)
		if err != nil || preview.Valid || preview.ErrorCode != "incompatibleBehavior" {
			t.Fatalf("incompatible rule accepted: %+v, %v", preview, err)
		}
	}
	_, _, err := prepareRuleSetEntry(RuleSetEntryInput{RuleSetID: ruleSetID}, State{
		RuleSets: []RuleSet{{ID: ruleSetID, SourceType: "http", Behavior: "classical"}},
	})
	if err != ruleEntryError("remoteRuleSet") {
		t.Fatalf("remote: %v", err)
	}
}

func TestRuleEntryRejectsStaleDraftAndRollsBackFailedValidation(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSetEntryValidated(RuleSetEntryInput{Name: "Existing", MatchType: "domain", Value: "example.com"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.file)
	if err != nil {
		t.Fatal(err)
	}
	input := RuleSetEntryInput{RuleSetID: state.RuleSets[0].ID, ExpectedRevision: 0, MatchType: "domain", Value: "anthropic.com"}
	if _, err := store.SaveRuleSetEntryValidated(input, nil); err != ruleEntryError("resourcesChanged") {
		t.Fatalf("stale draft saved: %v", err)
	}
	input.ExpectedRevision = state.Revision
	called := false
	rejected := errors.New("candidate configuration rejected")
	if _, err := store.SaveRuleSetEntryValidated(input, func(candidate State) error {
		called = true
		if got := candidate.RuleSets[0].Payload; !reflect.DeepEqual(got, []string{"DOMAIN-SUFFIX,example.com", "DOMAIN-SUFFIX,anthropic.com"}) {
			t.Fatalf("validator did not receive appended candidate: %v", got)
		}
		return rejected
	}); !errors.Is(err, rejected) || !called {
		t.Fatalf("candidate validation not enforced: %v", err)
	}
	after, err := os.ReadFile(store.file)
	if err != nil || string(before) != string(after) {
		t.Fatal("rejection changed the saved library", err)
	}
	updated, err := store.SaveRuleSetEntryValidated(input, nil)
	if err != nil || updated.Revision != state.Revision+1 {
		t.Fatalf("retry: %+v, %v", updated, err)
	}
}
