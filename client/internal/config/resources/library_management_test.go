package resources

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestRemovedDescriptionsReadWithoutRewritingAndStayOutOfExports(t *testing.T) {
	s, _ := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	state, err := s.SaveRuleSet(RuleSet{Name: "Rules", Emoji: "📄", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload: ['+.example.test']\n"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.file)
	raw = bytes.Replace(raw, []byte(`"name": "Rules"`), []byte(`"name": "Rules", "description": "old display metadata"`), 1)
	if err := os.WriteFile(s.file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.State()
	if err != nil || !reflect.DeepEqual(state, loaded) {
		t.Fatal("description prevented reading", err)
	}
	after, _ := os.ReadFile(s.file)
	if !bytes.Equal(raw, after) {
		t.Fatal("ordinary read rewrote the library")
	}
	encoded, _ := json.Marshal(loaded.RuleSets[0])
	if bytes.Contains(encoded, []byte(`"description"`)) {
		t.Fatal("removed field exported")
	}
	if _, err := s.SaveRuleSet(loaded.RuleSets[0]); err != nil {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(s.file)
	if bytes.Contains(after, []byte(`"description"`)) {
		t.Fatal("explicit save retained removed field")
	}
	for _, target := range []any{&RuleSet{}, &StrategyGroup{}} {
		if err := json.Unmarshal([]byte(`{"name":"Example","description":"discard","unknown":true}`), target); err == nil {
			t.Fatal("unrelated unknown field accepted")
		}
		if err := json.Unmarshal([]byte(`{"name":"Example","description":"discard","emoji":"🧠"}`), target); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBatchDeleteIsAtomicAndChecksReferencesAndRevision(t *testing.T) {
	s, _ := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	first, err := s.SaveRuleSet(RuleSet{Name: "Referenced", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload: ['+.example.test']\n"})
	if err != nil {
		t.Fatal(err)
	}
	referenced := first.RuleSets[0].ID
	second, err := s.SaveRuleSet(RuleSet{Name: "Unused", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload: ['+.example.org']\n"})
	if err != nil {
		t.Fatal(err)
	}
	unused := second.RuleSets[1].ID
	before, err := s.SaveStrategyGroup(StrategyGroup{Name: "Disabled owner", Kind: "rule", Disabled: true, Policy: PolicyTarget{Mode: "direct"}, RuleSetReferences: []RuleSetReference{{RuleSetID: referenced}}})
	if err != nil {
		t.Fatal(err)
	}
	input := DeleteInput{Kind: "rules", IDs: []string{unused, referenced}, ExpectedRevision: before.Revision}
	if _, err := s.DeleteValidated(input, nil); err == nil {
		t.Fatal("referenced rule deleted")
	}
	after, _ := s.State()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("part of rejected batch was deleted")
	}
	input.IDs = []string{unused}
	if _, err := s.DeleteValidated(input, func(State) error { return errors.New("candidate rejected") }); err == nil {
		t.Fatal("ignored validator")
	}
	after, _ = s.State()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed candidate was persisted")
	}
	input.ExpectedRevision--
	if _, err := s.DeleteValidated(input, nil); err == nil {
		t.Fatal("stale deletion accepted")
	}
	input.ExpectedRevision = before.Revision
	after, err = s.DeleteValidated(input, nil)
	if err != nil || len(after.RuleSets) != 1 || after.RuleSets[0].ID != referenced || after.Revision != before.Revision+1 {
		t.Fatal("atomic deletion failed", err)
	}
	input = DeleteInput{Kind: "groups", IDs: []string{after.StrategyGroups[0].ID}, ExpectedRevision: after.Revision}
	after, err = s.DeleteValidated(input, nil)
	if err != nil || len(after.StrategyGroups) != 0 || len(after.RuleSets) != 1 {
		t.Fatal("group deletion cascaded", err)
	}
}
