package resources

import (
	"bytes"
	"strings"
	"testing"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
	configinspect "jeemi/internal/config/inspect"
)

const reconstructionSource = `proxies: [{name: node, type: ss}]
proxy-groups: [{name: Source, type: select, proxies: [node]}]
proxy-providers: {nodes: {type: http, url: 'https://example.test/nodes'}}
rule-providers: {upstream: {type: inline, behavior: domain, payload: [source.test]}}
rules: ['RULE-SET,upstream,Source', 'MATCH,Source', 'DOMAIN,after.test,REJECT']
sub-rules: {old: ['MATCH,Source']}
x-unknown: {keep: true}
`

func localMatchFixture() (Plan, State) {
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{ruleGroupID, selectorID}
	plan.DefaultProxySelectorID = selectorID
	state := State{
		StrategyGroups: []StrategyGroup{
			{ID: selectorID, Name: "Local", Kind: StrategyGroupKindSelector, Type: "select", RuleOutput: RuleOutputRuleSet},
			{ID: ruleGroupID, Name: "Proxy rules", Kind: StrategyGroupKindRule, Policy: PolicyTarget{Mode: PolicyProxy}, RuleOutput: RuleOutputRuleSet, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleSetID}}},
		},
		RuleSets: []RuleSet{{ID: ruleSetID, Name: "local-rules", SourceType: "inline", Behavior: "domain", Payload: []string{"local.test"}}},
	}
	return plan, state
}

func TestLocalMatchModesDoNotChangeMatchingRuleTargets(t *testing.T) {
	for _, mode := range []string{"none", "direct", "selector"} {
		for _, strategy := range []compose.Strategy{compose.StrategyPrepend, compose.StrategyAppendBeforeTerminal} {
			t.Run(mode+"/"+string(strategy), func(t *testing.T) {
				plan, state := localMatchFixture()
				plan.RuleStrategy = strategy
				plan.Match.Mode = mode
				wantMatch := "MATCH,Source"
				if mode == "direct" {
					wantMatch = "MATCH,DIRECT"
				} else if mode == "selector" {
					plan.Match.SelectorID = selectorID
					wantMatch = "MATCH,Local"
				}
				result, err := Apply([]byte(reconstructionSource), state, plan)
				if err != nil {
					t.Fatal(err)
				}
				parsed, _ := document.Parse(result)
				rules, _, _ := document.Find(document.Root(parsed), "/rules")
				localIndex := 0
				if strategy == compose.StrategyAppendBeforeTerminal {
					localIndex = 1
				}
				if len(rules.Content) != 4 || rules.Content[localIndex].Value != "RULE-SET,local-rules,Local" || rules.Content[2].Value != wantMatch || rules.Content[3].Value != "DOMAIN,after.test,REJECT" {
					t.Fatalf("MATCH changed matching targets or terminal boundary: %s", result)
				}
				if err := configinspect.ValidateReferences(result); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCompleteReconstructionRemovesSourceRoutingAndKeepsNodeSources(t *testing.T) {
	for _, match := range []LocalMatch{{Mode: "direct"}, {Mode: "selector", SelectorID: selectorID}} {
		plan, state := localMatchFixture()
		plan.RuleStrategy, plan.Match = compose.StrategyReplace, match
		base, err := PrepareSource([]byte(reconstructionSource), plan)
		if err != nil {
			t.Fatal(err)
		}
		result, err := Apply(base, state, plan)
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := document.Parse(result)
		root := document.Root(parsed)
		for _, path := range []string{"/proxies", "/proxy-providers/nodes", "/x-unknown/keep", "/rule-providers/local-rules"} {
			if _, found, _ := document.Find(root, path); !found {
				t.Fatal("reconstruction lost preserved or generated data", path)
			}
		}
		if strings.Contains(string(result), "Source") || strings.Contains(string(result), "upstream") || strings.Contains(string(result), "after.test") {
			t.Fatalf("reconstruction retained old routing: %s", result)
		}
		if _, found, _ := document.Find(root, "/sub-rules"); found {
			t.Fatal("source sub-rules survived complete reconstruction")
		}
		rules, _, _ := document.Find(root, "/rules")
		want := "MATCH,DIRECT"
		if match.Mode == "selector" {
			want = "MATCH,Local"
		}
		if len(rules.Content) != 2 || rules.Content[1].Value != want {
			t.Fatal("full reconstruction did not produce exactly one explicit MATCH")
		}
		if err := configinspect.ValidateReferences(result); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRulePlanDerivesModesAndRejectsInvalidLocalMatch(t *testing.T) {
	plan, state := localMatchFixture()
	normalized, err := ValidatePlan(plan, state)
	if err != nil || normalized.selectorStrategy() != compose.StrategyAppend || normalized.ruleProviderStrategy() != compose.StrategyMergeByName {
		t.Fatal("merge mode did not preserve subscription resources", err)
	}
	plan.RuleStrategy = compose.StrategyReplace
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("full reconstruction accepted an inherited MATCH")
	}
	plan.Match = LocalMatch{Mode: "selector", SelectorID: selectorID}
	normalized, err = ValidatePlan(plan, state)
	if err != nil || normalized.selectorStrategy() != compose.StrategyReplace || normalized.ruleProviderStrategy() != compose.StrategyReplace {
		t.Fatal("full reconstruction did not replace all routing resources", err)
	}
	plan.DisabledStrategyGroupIDs = []string{selectorID}
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("MATCH accepted a disabled selector")
	}
	plan.DisabledStrategyGroupIDs = nil
	plan.StrategyGroupIDs = []string{ruleGroupID}
	plan.RuleStrategy = compose.StrategyPrepend
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("MATCH accepted a default-only, unordered selector")
	}
	plan.RulesDisabled = true
	base := []byte(reconstructionSource)
	prepared, err := PrepareSource(base, plan)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(prepared, state, plan)
	if err != nil || !bytes.Equal(result, base) {
		t.Fatal("disabled local rules applied MATCH or resolved unused references", err)
	}
}
