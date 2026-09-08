package resources

import (
	"strings"
	"testing"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
)

func TestSelectorReplacementRequiresExplicitSelectorAndDeduplicatesMembers(t *testing.T) {
	id := strings.Repeat("a", 32)
	selector := StrategyGroup{ID: id, Name: "Local", Kind: StrategyGroupKindSelector, Type: "select"}
	state := State{StrategyGroups: []StrategyGroup{selector}}
	plan := DefaultPlan()
	plan.RuleStrategy = compose.StrategyReplace
	plan.Match = LocalMatch{Mode: "direct"}
	plan.DefaultProxySelectorID = id
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("implicit default selector authorized complete replacement")
	}
	plan.StrategyGroupIDs = []string{id}
	// Repeated names cannot appear twice in a materialized selector, even
	// when different source entries match the same positive conditions.
	selectors, err := materializeSelectors([]string{id}, map[string]StrategyGroup{id: selector}, []proxyRecord{{Name: "🇺🇸 US", Type: "ss"}, {Name: "🇺🇸 US", Type: "ss"}, {Name: "🇯🇵 JP", Type: "ss"}})
	if err != nil {
		t.Fatal(err)
	}
	members, _, _ := document.Find(selectors.Content[0], "/proxies")
	if len(members.Content) != 2 {
		t.Fatal("duplicate subscription nodes were included twice")
	}
	if _, err := ValidatePlan(plan, state); err != nil {
		t.Fatal(err)
	}
}

func TestRuleGroupEmojiIsMetadataAndKindIsImmutable(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSet(RuleSet{Name: "Rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload: [DOMAIN,example.org]"})
	// Use a quoted classical payload; the unquoted two-item fixture must fail.
	if err == nil {
		t.Fatal("malformed classical payload accepted")
	}
	state, err = store.SaveRuleSet(RuleSet{Name: "Rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload: ['DOMAIN,example.org']"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.SaveStrategyGroup(StrategyGroup{Name: "Rule group", Emoji: "🚀", Kind: "rule", Policy: PolicyTarget{Mode: "direct"}, RuleSetReferences: []RuleSetReference{{RuleSetID: state.RuleSets[0].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	group := state.StrategyGroups[0]
	if group.Emoji != "🚀" {
		t.Fatal("rule group emoji was lost")
	}
	group.Kind, group.Type = "selector", "select"
	if _, err := store.SaveStrategyGroup(group); err == nil {
		t.Fatal("editing changed the existing group kind")
	}
}

func TestInvalidSavedClassicalMRSRemainsReadableAndRepairable(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveRuleSet(RuleSet{Name: "Remote", SourceType: "http", Behavior: "classical", Format: "yaml", URL: "https://example.test/rules"})
	if err != nil {
		t.Fatal(err)
	}
	// An invalid saved definition must remain editable for repair.
	state.RuleSets[0].Format = "mrs"
	if _, err := store.saveUnlocked(state); err != nil {
		t.Fatal(err)
	}
	state, err = store.State()
	if err != nil {
		t.Fatal("invalid saved definition became unreadable", err)
	}
	if _, err := store.SaveRuleSet(state.RuleSets[0]); err == nil {
		t.Fatal("invalid candidate format accepted")
	}
	state.RuleSets[0].Format = "yaml"
	if _, err := store.SaveRuleSet(state.RuleSets[0]); err != nil {
		t.Fatal("saved definition cannot be repaired", err)
	}
}

func TestDisabledRulesSkipAllResourcesWithoutProxyTarget(t *testing.T) {
	ruleID, groupID := strings.Repeat("c", 32), strings.Repeat("b", 32)
	state := State{
		RuleSets:       []RuleSet{{ID: ruleID, Name: "Local", SourceType: "inline", Behavior: "domain", Payload: []string{"example.org"}}},
		StrategyGroups: []StrategyGroup{{ID: groupID, Kind: StrategyGroupKindRule, Policy: PolicyTarget{Mode: PolicyProxy}, RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleID}}}},
	}
	plan := DefaultPlan()
	plan.StrategyGroupIDs, plan.RulesDisabled = []string{groupID}, true
	base := []byte("rules: ['DOMAIN,source.test,DIRECT', 'MATCH,DIRECT']\n")
	output, err := Apply(base, state, plan)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := document.Parse(output)
	rules, _, _ := document.Find(document.Root(parsed), "/rules")
	_, found, _ := document.Find(document.Root(parsed), "/rule-providers/Local")
	if found || string(output) != string(base) || len(rules.Content) != 2 {
		t.Fatal("disabled local rules modified the original configuration")
	}
}
