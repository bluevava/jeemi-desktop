package application

import (
	"jeemi/internal/config/resources"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
	"reflect"
	"testing"
)

func TestLibraryAssociationsAndBatchDeletionRecheckCurrentReferences(t *testing.T) {
	s, driver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	first, err := s.SaveStrategyGroup(resources.StrategyGroup{Name: "Referenced", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	referenced := first.StrategyGroups[0].ID
	second, err := s.SaveStrategyGroup(resources.StrategyGroup{Name: "Unused", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	unused := second.StrategyGroups[1].ID
	plan := resources.DefaultPlan()
	plan.StrategyGroupIDs = []string{referenced}
	plan.RulesDisabled = true
	config, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Configuration", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.ConfigReferences) != 1 || before.ConfigReferences[0].ID != config.ID || !reflect.DeepEqual(before.ConfigReferences[0].StrategyGroupIDs, []string{referenced}) {
		t.Fatal("lost disabled association")
	}
	if _, err := s.DeleteLocalConfigResources(resources.DeleteInput{Kind: "groups", IDs: []string{unused, referenced}, ExpectedRevision: before.Revision}); err == nil {
		t.Fatal("deleted referenced group")
	}
	after, _ := s.LocalConfigResources()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rejected batch changed library")
	}
	// A configuration link can change without changing the resource revision.
	plan.StrategyGroupIDs = append(plan.StrategyGroupIDs, unused)
	if _, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: config.ID, Name: config.Name, ResourcePlan: plan}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteLocalConfigResources(resources.DeleteInput{Kind: "groups", IDs: []string{unused}, ExpectedRevision: before.Revision}); err == nil {
		t.Fatal("missed concurrent configuration reference")
	}
	if driver.starts.Load() != 0 {
		t.Fatal("library management started proxy")
	}
}

func TestProcessPresetsCreateOnlySelectedPlatformAndKeepSeparateOwners(t *testing.T) {
	s, _ := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	for _, platform := range []string{"windows", "darwin", "linux"} {
		s.runtime.Platform.OS = platform
		config, err := s.CreateLocalConfigPreset("process")
		if err != nil {
			t.Fatal(err)
		}
		state, err := s.LocalConfigResources()
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, group := range state.StrategyGroups {
			included := false
			for _, id := range config.ResourcePlan.StrategyGroupIDs {
				included = included || group.ID == id
			}
			if !included {
				continue
			}
			for _, ref := range group.RuleSetReferences {
				for _, rule := range state.RuleSets {
					if rule.ID == ref.RuleSetID && rule.Platform != "" {
						count++
						if rule.Platform != platform {
							t.Fatal("foreign platform bound")
						}
					}
				}
			}
		}
		if count != 1 {
			t.Fatal("platform rule count", count)
		}
	}
}
