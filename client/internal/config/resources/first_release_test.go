package resources

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func firstReleaseLibrary(t *testing.T) (*Store, State, []byte) {
	t.Helper()
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSet(RuleSet{Name: "Rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload:\n  # preserved\n  - DOMAIN,example.org\n"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.SaveStrategyGroup(StrategyGroup{
		Name: "Selector", Kind: StrategyGroupKindSelector, Type: "select",
		RuleSetReferences: []RuleSetReference{{RuleSetID: state.RuleSets[0].ID}},
		Filter:            NodeFilter{ProxyTypes: []string{"ss"}, NamePatterns: []string{"🇯🇵", "!test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.file)
	if err != nil {
		t.Fatal(err)
	}
	return store, state, data
}

func TestFirstReleaseLibraryRoundTripDoesNotRewrite(t *testing.T) {
	store, saved, before := firstReleaseLibrary(t)
	var document libraryDocument
	if err := json.Unmarshal(before, &document); err != nil {
		t.Fatal(err)
	}
	if document.Version != 1 {
		t.Fatal("new library is not version 1")
	}
	for range 2 {
		state, err := store.State()
		if err != nil {
			t.Fatal(err)
		}
		if state.Revision != saved.Revision || state.RuleSets[0].PayloadYAML != saved.RuleSets[0].PayloadYAML || len(state.StrategyGroups[0].Filter.NamePatterns) != 2 {
			t.Fatal("library read changed its revision, YAML or filters")
		}
	}
	after, err := os.ReadFile(store.file)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("library read rewrote persisted content", err)
	}
}

func TestFirstReleaseLibraryRejectsHistoricalFieldsWithoutRewriting(t *testing.T) {
	cases := map[string]func(map[string]any){
		"version": func(value map[string]any) { value["version"] = 6 },
		"country filter": func(value map[string]any) {
			value["strategyGroups"].([]any)[0].(map[string]any)["filter"].(map[string]any)["countries"] = []string{"JP"}
		},
		"reference no-resolve": func(value map[string]any) {
			value["strategyGroups"].([]any)[0].(map[string]any)["ruleSetReferences"].([]any)[0].(map[string]any)["noResolve"] = true
		},
		"rule selector": func(value map[string]any) {
			value["strategyGroups"].([]any)[0].(map[string]any)["policy"].(map[string]any)["selectorId"] = selectorID
		},
		"migration state": func(value map[string]any) {
			value["legacyRuleProxyTargets"] = map[string]string{ruleGroupID: selectorID}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			store, _, data := firstReleaseLibrary(t)
			var value map[string]any
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.State(); err == nil {
				t.Fatal("historical resource data was accepted")
			}
			after, err := os.ReadFile(store.file)
			if err != nil || !bytes.Equal(after, data) {
				t.Fatal("invalid library was rewritten", err)
			}
		})
	}
}
