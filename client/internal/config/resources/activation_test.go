package resources

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestResourceActivationOmitsRulesProvidersAndSelectorsWithoutMutatingDefinitions(t *testing.T) {
	for _, output := range []string{RuleOutputRuleSet, RuleOutputInline} {
		t.Run(output, func(t *testing.T) {
			groupID, ruleID := strings.Repeat("a", 32), strings.Repeat("b", 32)
			state := State{
				StrategyGroups: []StrategyGroup{{ID: groupID, Name: "Media", Kind: "selector", Type: "select", AutoSelect: true, RuleOutput: output, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleID}}}},
				RuleSets:       []RuleSet{{ID: ruleID, Name: "Instagram", SourceType: "inline", Behavior: "domain", Payload: []string{"+.instagram.com"}}},
			}
			plan := DefaultPlan()
			plan.StrategyGroupIDs = []string{groupID}
			base := []byte("proxies: [{name: JP, type: ss}]\nrules: ['MATCH,DIRECT']\n")
			initial, err := Apply(base, state, plan)
			if err != nil {
				t.Fatal(err)
			}
			for _, disableGroup := range []bool{false, true} {
				state.StrategyGroups[0].Disabled = disableGroup
				state.RuleSets[0].Disabled = !disableGroup
				before, _ := json.Marshal(state)
				result, err := Apply(base, state, plan)
				if err != nil {
					t.Fatal(err)
				}
				after, _ := json.Marshal(state)
				if !bytes.Equal(before, after) {
					t.Fatal("activation changed saved definitions")
				}
				if strings.Contains(string(result), "Instagram") || strings.Contains(string(result), "instagram.com") {
					t.Fatal("disabled resource emitted provider or rule")
				}
				if strings.Contains(string(result), "Media Auto") == disableGroup {
					t.Fatal("group/Auto lifecycle does not match switch")
				}
				if count, err := RuleProviderCount(plan, state); err != nil || count != 0 {
					t.Fatal("inactive provider counted", count, err)
				}
			}
			state.StrategyGroups[0].Disabled = false
			state.RuleSets[0].Disabled = false
			restored, err := Apply(base, state, plan)
			if err != nil || !bytes.Equal(initial, restored) {
				t.Fatal("reenabling lost original routing", err)
			}
			plan.DisabledStrategyGroupIDs = []string{groupID}
			result, err := Apply(base, state, plan)
			if err != nil || strings.Contains(string(result), "Media") {
				t.Fatal("global switch overrode the local group switch", err)
			}
		})
	}
}

func TestPlatformRuleSetsOnlyEmitTheirOwnPayloadAndProvider(t *testing.T) {
	groupID := strings.Repeat("a", 32)
	state := State{StrategyGroups: []StrategyGroup{{ID: groupID, Name: "Processes", Kind: "rule", Policy: PolicyTarget{Mode: PolicyDirect}, RuleOutput: RuleOutputRuleSet}}}
	for i, platform := range []string{"windows", "darwin", "linux"} {
		rule := RuleSet{ID: strings.Repeat(string(rune('b'+i)), 32), Name: platform, Platform: platform, SourceType: "inline", Behavior: "classical", Payload: []string{"PROCESS-NAME," + platform + "-fixture"}}
		state.RuleSets = append(state.RuleSets, rule)
		state.StrategyGroups[0].RuleSetReferences = append(state.StrategyGroups[0].RuleSetReferences, RuleSetReference{RuleSetID: rule.ID})
	}
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{groupID}
	for _, platform := range []string{"windows", "darwin", "linux"} {
		result, err := Apply([]byte("rules: ['MATCH,DIRECT']"), state, plan, ApplyOptions{Platform: platform})
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Providers map[string]any `yaml:"rule-providers"`
			Rules     []string       `yaml:"rules"`
		}
		if err := yaml.Unmarshal(result, &config); err != nil {
			t.Fatal(err)
		}
		if len(config.Providers) != 1 || config.Providers[platform] == nil || !reflect.DeepEqual(config.Rules, []string{"RULE-SET," + platform + ",DIRECT", "MATCH,DIRECT"}) {
			t.Fatalf("%s emitted other platforms: %s", platform, result)
		}
		if count, err := RuleProviderCount(plan, state, platform); err != nil || count != 1 {
			t.Fatal(count, err)
		}
	}
	if _, err := Apply([]byte("{}"), state, plan); err == nil {
		t.Fatal("missing platform accepted")
	}
	state.StrategyGroups[0].Disabled = true
	if _, err := Apply([]byte("{}"), state, plan); err != nil {
		t.Fatal("disabled resources required a platform", err)
	}
	_, err := normalizeRuleSet(RuleSet{Name: "Invalid platform", Platform: "android", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload: ['PROCESS-NAME,fixture']"}, time.Now())
	if err == nil {
		t.Fatal("unsupported rule-set platform accepted")
	}
}

func TestDisabledDefaultSelectorIsRejectedButInactiveProxyRulesNeedNoTarget(t *testing.T) {
	selectorID, groupID, ruleID := strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 32)
	state := State{
		StrategyGroups: []StrategyGroup{
			{ID: selectorID, Name: "Proxy", Kind: "selector", Type: "select", Disabled: true},
			{ID: groupID, Name: "Apps", Kind: "rule", Policy: PolicyTarget{Mode: PolicyProxy}, RuleSetReferences: []RuleSetReference{{RuleSetID: ruleID}}},
		},
		RuleSets: []RuleSet{{ID: ruleID, Name: "Apps", SourceType: "inline", Behavior: "domain", Payload: []string{"+.example.com"}}},
	}
	plan := DefaultPlan()
	plan.StrategyGroupIDs = []string{selectorID, groupID}
	plan.DefaultProxySelectorID = selectorID
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("active rules accepted a disabled default selector")
	}
	state.RuleSets[0].Disabled = true
	if _, err := Apply([]byte("rules: ['MATCH,DIRECT']"), state, plan); err != nil {
		t.Fatal("inactive rules required a target", err)
	}
	plan.Match = LocalMatch{Mode: "selector", SelectorID: selectorID}
	if _, err := ValidatePlan(plan, state); err == nil {
		t.Fatal("MATCH accepted disabled selector")
	}
}
