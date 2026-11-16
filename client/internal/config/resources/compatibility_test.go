package resources

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestResourceExtensionsSurviveRenameReorderAndFalseToggle(t *testing.T) {
	store, state, raw := firstReleaseLibrary(t)
	var document map[string]any
	json.Unmarshal(raw, &document)
	document["futureLibrary"] = map[string]any{"enabled": false}
	group := document["strategyGroups"].([]any)[0].(map[string]any)
	group["futureGroup"] = "keep-group"
	group["filter"].(map[string]any)["futureFilter"] = true
	group["ruleSetReferences"].([]any)[0].(map[string]any)["futureReference"] = "keep-reference"
	document["ruleSets"].([]any)[0].(map[string]any)["futureRule"] = "keep-rule"
	raw, _ = json.Marshal(document)
	os.WriteFile(store.file, raw, 0600)
	for i := 0; i < 3; i++ {
		loaded, err := store.State()
		if err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(store.file)
		if !bytes.Equal(after, raw) {
			t.Fatal("read rewrote library")
		}
		edit := loaded.StrategyGroups[0]
		edit.Name = "Renamed"
		edit.Disabled = i%2 == 0
		state, err = store.SaveStrategyGroup(edit)
		if err != nil {
			t.Fatal(err)
		}
		rule := state.RuleSets[0]
		rule.Name = "Renamed rules"
		if _, err := store.SaveRuleSet(rule); err != nil {
			t.Fatal(err)
		}
		raw, _ = os.ReadFile(store.file)
		for _, key := range []string{"futureLibrary", "futureGroup", "futureFilter", "futureReference", "futureRule"} {
			if !bytes.Contains(raw, []byte(key)) {
				t.Fatal("lost extension", key)
			}
		}
	}
}
