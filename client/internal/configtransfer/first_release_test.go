package configtransfer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/config/resources"
	"jeemi/internal/profile"
)

func TestFirstReleasePackagePreservesCurrentRoutingAndRejectsHistoricalFields(t *testing.T) {
	groupID, ruleID := strings.Repeat("a", 32), strings.Repeat("b", 32)
	plan := resources.DefaultPlan()
	plan.StrategyGroupIDs = []string{groupID}
	plan.Match = resources.LocalMatch{Mode: "selector", SelectorID: groupID}
	state := resources.State{
		StrategyGroups: []resources.StrategyGroup{{ID: groupID, Name: "Selector", Kind: "selector", Type: "select", RuleSetReferences: []resources.RuleSetReference{{RuleSetID: ruleID}}, Filter: resources.NodeFilter{NamePatterns: []string{"🇯🇵", "!test"}}}},
		RuleSets:       []resources.RuleSet{{ID: ruleID, Name: "Rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload:\n  # keep comment\n  - DOMAIN,example.org\n", Payload: []string{"DOMAIN,example.org"}}},
	}
	p, err := FromConfig(profile.LocalConfig{LocalConfigSummary: profile.LocalConfigSummary{Name: "首发配置"}, ResourcePlan: plan, Fields: []profile.LocalConfigField{{Path: "/tcp-concurrent", ValueYAML: "true"}}}, state)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data, ConfigKind)
	if err != nil || !reflect.DeepEqual(p, decoded) {
		t.Fatal("portable configuration did not round-trip", err)
	}
	if decoded.Version != 1 || decoded.Config.ResourcePlan.Version != 1 {
		t.Fatal("export is not the first-release format")
	}
	for name, mutate := range map[string]func(map[string]any){
		"package version": func(value map[string]any) { value["version"] = 2 },
		"rule version": func(value map[string]any) {
			value["config"].(map[string]any)["resourcePlan"].(map[string]any)["version"] = 2
		},
		"missing rule version": func(value map[string]any) {
			delete(value["config"].(map[string]any)["resourcePlan"].(map[string]any), "version")
		},
		"independent mode": func(value map[string]any) {
			value["config"].(map[string]any)["resourcePlan"].(map[string]any)["ruleProviderStrategy"] = "replace"
		},
		"country": func(value map[string]any) {
			value["config"].(map[string]any)["strategyGroups"].([]any)[0].(map[string]any)["filter"].(map[string]any)["countries"] = []string{"JP"}
		},
		"reference flag": func(value map[string]any) {
			value["config"].(map[string]any)["strategyGroups"].([]any)[0].(map[string]any)["ruleSetReferences"].([]any)[0].(map[string]any)["noResolve"] = true
		},
	} {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			mutate(value)
			invalid, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Decode(invalid, ConfigKind); err == nil {
				t.Fatal("historical package shape was accepted")
			}
		})
	}
}
