package resources

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
)

func TestDisabledLocalRulesPreserveSourceAndUnresolvedReferences(t *testing.T) {
	base := []byte("# Keep routing and other fields verbatim.\nproxy-groups: [{name: Source, type: select, proxies: [DIRECT]}]\nrule-providers: {remote: {type: http, url: 'https://example.test/rules'}}\nrules: ['RULE-SET,remote,Source', 'MATCH,DIRECT']\nsub-rules: {nested: ['MATCH,Source']}\ntcp-concurrent: true\n")
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{strings.Repeat("a", 32)}
	plan.DefaultProxySelectorID = strings.Repeat("b", 32)
	plan.RuleStrategy = compose.StrategyReplace
	plan.RulesDisabled = true
	output, err := Apply(base, State{}, plan)
	if err != nil || !bytes.Equal(base, output) {
		t.Fatal("disabled resource layer touched the configuration or resolved inactive references", err)
	}
	if count, err := RuleProviderCount(plan, State{}); err != nil || count != 0 {
		t.Fatal("disabled references affected provider count", count, err)
	}
	normalized, err := NormalizePlan(plan)
	if err != nil || normalized.RuleStrategy != compose.StrategyReplace || normalized.StrategyGroupIDs[0] != plan.StrategyGroupIDs[0] {
		t.Fatal("disabled configuration lost saved mode or references", err)
	}
	plan.RulesDisabled = false
	if _, err := Apply(base, State{}, plan); err == nil {
		t.Fatal("re-enabling unresolved references bypassed validation")
	}
}

func TestReferenceSwitchSkipsSelectorProviderAndRulesAndRestoresOrder(t *testing.T) {
	selectorID, directID, firstRule, secondRule := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32), strings.Repeat("d", 32)
	state := State{
		StrategyGroups: []StrategyGroup{
			{ID: selectorID, Name: "Local", Kind: StrategyGroupKindSelector, Type: "select", RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{{RuleSetID: firstRule}}},
			{ID: directID, Name: "Direct rules", Kind: StrategyGroupKindRule, Policy: PolicyTarget{Mode: PolicyDirect}, RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{{RuleSetID: secondRule}}},
		},
		RuleSets: []RuleSet{
			{ID: firstRule, Name: "ProxySet", SourceType: "inline", Behavior: "domain", Payload: []string{"proxy.test"}},
			{ID: secondRule, Name: "DirectSet", SourceType: "inline", Behavior: "domain", Payload: []string{"direct.test"}},
		},
	}
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{selectorID, directID}
	plan.DefaultProxySelectorID = selectorID
	plan.DisabledStrategyGroupIDs = []string{selectorID}
	base := []byte("rules: ['MATCH,DIRECT']\n")
	output, err := Apply(base, state, plan)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := document.Parse(output)
	root := document.Root(parsed)
	if _, found, _ := document.Find(root, "/proxy-groups"); found {
		t.Fatal("default binding silently re-enabled disabled selector")
	}
	if _, found, _ := document.Find(root, "/rule-providers/ProxySet"); found {
		t.Fatal("disabled reference emitted a provider")
	}
	rules, _, _ := document.Find(root, "/rules")
	if len(rules.Content) != 2 || rules.Content[0].Value != "RULE-SET,DirectSet,DIRECT" {
		t.Fatal("disabled reference emitted rules or skipped an enabled rule")
	}
	if count, err := RuleProviderCount(plan, state); err != nil || count != 1 {
		t.Fatal("provider count included disabled group", count, err)
	}
	plan.DisabledStrategyGroupIDs = nil
	output, err = Apply(base, state, plan)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ = document.Parse(output)
	rules, _, _ = document.Find(document.Root(parsed), "/rules")
	if len(rules.Content) != 3 || rules.Content[0].Value != "RULE-SET,ProxySet,Local" || rules.Content[1].Value != "RULE-SET,DirectSet,DIRECT" {
		t.Fatal("re-enabling group lost its original order")
	}
}

func TestDisabledSelectorCannotServeActiveProxyRulesOrAuthorizeReplacement(t *testing.T) {
	selectorID, ruleID := strings.Repeat("a", 32), strings.Repeat("b", 32)
	state := State{StrategyGroups: []StrategyGroup{
		{ID: selectorID, Name: "Local", Kind: StrategyGroupKindSelector, Type: "select"},
		{ID: ruleID, Name: "Proxy rules", Kind: StrategyGroupKindRule, Policy: PolicyTarget{Mode: PolicyProxy}},
	}}
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{selectorID, ruleID}
	plan.DisabledStrategyGroupIDs = []string{selectorID}
	plan.DefaultProxySelectorID = selectorID
	if _, err := ValidatePlan(plan, state); err == nil || !strings.Contains(err.Error(), "disabled selector") {
		t.Fatal("active proxy rules accepted a disabled target", err)
	}
	plan.DisabledStrategyGroupIDs = []string{selectorID, ruleID}
	if _, err := ValidatePlan(plan, state); err != nil {
		t.Fatal("disabled dependency blocked saving other settings", err)
	}
	plan.RuleStrategy = compose.StrategyReplace
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("disabled selector authorized replacing all source selectors")
	}
	plan.DisabledStrategyGroupIDs = []string{selectorID, selectorID}
	if _, err := NormalizePlan(plan); err == nil {
		t.Fatal("duplicate disabled reference accepted")
	}
	plan.DisabledStrategyGroupIDs = []string{strings.Repeat("c", 32)}
	if _, err := NormalizePlan(plan); err == nil {
		t.Fatal("unreferenced disabled resource accepted")
	}
}

func TestImportRemapsDisabledReferencesAndExportsTheirDependencies(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	groupID, ruleID := strings.Repeat("a", 32), strings.Repeat("b", 32)
	group := StrategyGroup{ID: groupID, Name: "Imported", Kind: StrategyGroupKindSelector, Type: "select", RuleSetReferences: []RuleSetReference{{RuleSetID: ruleID}}}
	rule := RuleSet{ID: ruleID, Name: "Imported rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload: ['DOMAIN,example.test']"}
	plan := DefaultPlan()
	plan.StrategyGroupIDs, plan.DisabledStrategyGroupIDs = []string{groupID}, []string{groupID}
	plan.RulesDisabled = true
	plan.RuleStrategy = compose.StrategyPrepend
	plan.Match = LocalMatch{Mode: "selector", SelectorID: groupID}
	candidate, err := store.PrepareImport(plan, []StrategyGroup{group}, []RuleSet{rule})
	if err != nil {
		t.Fatal(err)
	}
	if !candidate.Plan.RulesDisabled || candidate.Plan.RuleStrategy != compose.StrategyPrepend || candidate.Plan.StrategyGroupIDs[0] == groupID || candidate.Plan.Match.SelectorID != candidate.Plan.StrategyGroupIDs[0] || !reflect.DeepEqual(candidate.Plan.StrategyGroupIDs, candidate.Plan.DisabledStrategyGroupIDs) {
		t.Fatal("import lost activation state or retained old disabled IDs")
	}
	groups, rules, err := ExportGraph(candidate.Plan, candidate.State)
	if err != nil || len(groups) != 1 || len(rules) != 1 || groups[0].RuleSetReferences[0].RuleSetID != rules[0].ID {
		t.Fatal("export lost disabled resources", err)
	}
	if !reflect.DeepEqual(plan.DisabledStrategyGroupIDs, []string{groupID}) {
		t.Fatal("import mutated its package input")
	}
}
