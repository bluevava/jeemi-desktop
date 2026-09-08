package resources

import (
	"reflect"
	"strings"
	"testing"
)

func TestImportRejectsOwnershipConflictsAndAmbiguousPackages(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSet(RuleSet{Name: "Shared", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload: ['DOMAIN,example.test']"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.SaveStrategyGroup(StrategyGroup{Name: "Existing owner", Kind: "rule", Policy: PolicyTarget{Mode: "direct"}, RuleSetReferences: []RuleSetReference{{RuleSetID: state.RuleSets[0].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	rule := state.RuleSets[0]
	group := state.StrategyGroups[0]
	group.ID, group.Name = strings.Repeat("a", 32), "Different owner"
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{group.ID}
	if _, err := store.PrepareImport(plan, []StrategyGroup{group}, []RuleSet{rule}); err == nil || !strings.Contains(err.Error(), "one strategy group") {
		t.Fatal("rule-set ownership stolen", err)
	}
	after, _ := store.State()
	if !reflect.DeepEqual(state, after) {
		t.Fatal("rejected import wrote library")
	}
	group.Name = "Existing owner"
	validGroups, validRules := []StrategyGroup{group}, []RuleSet{rule}
	if _, err := store.PrepareImport(plan, validGroups, validRules); err != nil {
		t.Fatal("same-name owner replacement rejected", err)
	}
	for _, tc := range []struct {
		name   string
		groups []StrategyGroup
		rules  []RuleSet
	}{
		{"missing rule", validGroups, nil},
		{"duplicate group", []StrategyGroup{group, group}, validRules},
		{"duplicate rule", validGroups, []RuleSet{rule, rule}},
		{"unrelated resource", nil, validRules},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.PrepareImport(plan, tc.groups, tc.rules); err == nil {
				t.Fatal("invalid package accepted")
			}
		})
	}
}
