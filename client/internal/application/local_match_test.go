package application

import (
	"strings"
	"testing"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
	"jeemi/internal/config/fallbackoverride"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	"jeemi/internal/profile"
)

func TestLocalMatchPrecedesSubscriptionFallbackAndSurvivesSelectorRename(t *testing.T) {
	s, _, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	library, err := s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Local", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	group := library.StrategyGroups[0]
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{group.ID}
	plan.Match = configresources.LocalMatch{Mode: "selector", SelectorID: group.ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local MATCH", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(id, local.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		selection fallbackoverride.Selection
		target    string
	}{
		{fallbackoverride.Selection{Mode: "direct"}, "DIRECT"},
		{fallbackoverride.Selection{Mode: "none"}, "Local"},
		{fallbackoverride.Selection{Mode: "selector", Selector: "Hidden"}, "Hidden"},
	} {
		detail, _ := s.Subscription(id)
		result, err := s.SetSubscriptionFallback(id, tc.selection, detail.FallbackOverrideRevision)
		if err != nil || result.State.Projection.Fallback.OriginalTarget != "Local" {
			t.Fatal("subscription fallback did not inspect the locally processed MATCH", err)
		}
		view, err := s.RuntimeConfigurationText()
		if err != nil || !strings.Contains(view.Contents, "MATCH,"+tc.target) {
			t.Fatal("incorrect fallback precedence", tc.target, err)
		}
	}
	group.Name = "Renamed"
	if _, err := s.SaveStrategyGroup(group); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Subscription(id)
	result, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: "none"}, detail.FallbackOverrideRevision)
	if err != nil || result.State.Projection.Fallback.OriginalTarget != "Renamed" {
		t.Fatal("local MATCH was not bound by stable selector ID", err)
	}
	// Custom sub-rules are injected after source routing is removed.
	plan.RuleStrategy = compose.StrategyReplace
	local, err = s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: local.Name, ResourcePlan: plan, Fields: []profile.LocalConfigField{
		{Path: "/sub-rules", ValueYAML: "custom: ['MATCH,Renamed']", Strategy: compose.StrategyReplace},
		{Path: "/tcp-concurrent", ValueYAML: "true", Strategy: compose.StrategyReplace},
	}})
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || strings.Contains(view.Contents, "Original") || strings.Contains(view.Contents, "Hidden") || !strings.Contains(view.Contents, "tcp-concurrent: true") {
		t.Fatal("reconstruction retained source groups or lost custom fields", err)
	}
	parsed, _ := document.Parse([]byte(view.Contents))
	if _, found, _ := document.Find(document.Root(parsed), "/sub-rules/custom"); !found {
		t.Fatal("source cleanup removed a custom sub-rule")
	}
	plan.DisabledStrategyGroupIDs = []string{group.ID}
	if _, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: local.Name, ResourcePlan: plan}); err == nil {
		t.Fatal("disabled local MATCH selector was saved")
	}
	after, err := s.LocalConfig(local.ID)
	if err != nil || after.Revision != local.Revision {
		t.Fatal("invalid MATCH edit replaced the last valid revision", err)
	}
	source, err := s.SubscriptionText(id)
	if err != nil || source.Contents != fallbackFixture {
		t.Fatal("local reconstruction rewrote immutable subscription source", err)
	}
}

func TestLocalMatchPackageRemapsTargetAndPreservesModernReconstruction(t *testing.T) {
	source, _, _ := fallbackService(t)
	local := portableFixture(t, source)
	plan := local.ResourcePlan
	plan.StrategyGroupIDs = append(plan.StrategyGroupIDs, plan.DefaultProxySelectorID)
	plan.RuleStrategy = compose.StrategyReplace
	plan.Match = configresources.LocalMatch{Mode: "selector", SelectorID: plan.DefaultProxySelectorID}
	local, err := source.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: local.Name, Fields: local.Fields, ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := source.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	if err != nil {
		t.Fatal(err)
	}
	target, _, subscriptions := fallbackService(t)
	current, err := target.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Import target"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.SetSubscriptionLocalConfig(subscriptions.SelectedSubscriptionID, current.ID); err != nil {
		t.Fatal(err)
	}
	preview, err := target.PreviewLocalPackage(configtransfer.ConfigKind, current.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.ResolveLocalPackage(preview.Token, true); err != nil {
		t.Fatal(err)
	}
	imported, err := target.LocalConfig(current.ID)
	if err != nil || imported.ResourcePlan.Match.SelectorID == plan.Match.SelectorID || imported.ResourcePlan.Match.SelectorID != imported.ResourcePlan.StrategyGroupIDs[1] || imported.ResourcePlan.Version != configresources.CurrentPlanVersion {
		t.Fatal("package failed to remap local MATCH or modern mode", err)
	}
	view, err := target.RuntimeConfigurationText()
	if err != nil || !strings.Contains(view.Contents, "MATCH,Selector") || strings.Contains(view.Contents, "MATCH,Original") {
		t.Fatal("imported local MATCH was not applied", err)
	}
}
