package application

import (
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/config/compose"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/profile"
)

func TestLocalRuleSwitchesPreserveCustomFieldsAndRejectInvalidReactivation(t *testing.T) {
	for _, scope := range []string{"all", "group"} {
		t.Run(scope, func(t *testing.T) {
			s, _, subscriptions := fallbackService(t)
			// A collision only becomes invalid when this local selector is emitted.
			library, err := s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Original", Kind: "selector", Type: "select"})
			if err != nil {
				t.Fatal(err)
			}
			groupID := library.StrategyGroups[0].ID
			plan := configresources.DefaultPlan()
			plan.StrategyGroupIDs = []string{groupID}
			plan.DefaultProxySelectorID = groupID
			plan.RuleStrategy = compose.StrategyPrepend
			if scope == "all" {
				plan.RulesDisabled = true
			} else {
				plan.DisabledStrategyGroupIDs = []string{groupID}
			}
			input := profile.SaveLocalConfigInput{Name: "Switches", ResourcePlan: plan, Fields: []profile.LocalConfigField{{Path: "/tcp-concurrent", ValueYAML: "true", Strategy: compose.StrategyReplace}}}
			local, err := s.SaveLocalConfig(input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.SetSubscriptionLocalConfig(subscriptions.SelectedSubscriptionID, local.ID); err != nil {
				t.Fatal("inactive group blocked associating custom fields", err)
			}
			view, err := s.RuntimeConfigurationText()
			if err != nil || !strings.Contains(view.Contents, "tcp-concurrent: true") || !strings.Contains(view.Contents, "MATCH,Original") {
				t.Fatal("switch lost custom fields or source routing", err)
			}
			data, _, err := s.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
			if err != nil {
				t.Fatal(err)
			}
			p, err := configtransfer.Decode(data, configtransfer.ConfigKind)
			if err != nil || !reflect.DeepEqual(p.Config.ResourcePlan, local.ResourcePlan) || len(p.Config.StrategyGroups) != 1 {
				t.Fatal("package lost switches or inactive references", err)
			}
			if _, err := s.DeleteStrategyGroup(groupID); err == nil {
				t.Fatal("disabled reference allowed deleting a saved resource")
			}
			input.ID = local.ID
			input.ResourcePlan.RulesDisabled = false
			input.ResourcePlan.DisabledStrategyGroupIDs = nil
			if _, err := s.SaveLocalConfig(input); err == nil {
				t.Fatal("invalid reactivation bypassed candidate check")
			}
			after, err := s.LocalConfig(local.ID)
			if err != nil || !reflect.DeepEqual(after, local) {
				t.Fatal("failed reactivation changed the saved switches", err)
			}
			afterView, err := s.RuntimeConfigurationText()
			if err != nil || afterView.Contents != view.Contents {
				t.Fatal("failed reactivation replaced resolved configuration", err)
			}
			if s.runtimeManager.Status().State != mihomoruntime.StateStopped {
				t.Fatal("switching local rules started the core")
			}
		})
	}
}
