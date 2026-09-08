package application

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
	"jeemi/internal/subscription"
)

func portableFixture(t *testing.T, s *Service) profile.LocalConfig {
	t.Helper()
	refs := []resources.RuleSetReference{}
	for _, name := range []string{"RoutingRules", "DefaultRules"} {
		state, err := s.SaveRuleSet(resources.RuleSet{Name: name, SourceType: "inline", Behavior: "classical", PayloadYAML: "payload:\n  # Keep this comment\n  - DOMAIN,example.test\n"})
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range state.RuleSets {
			if rule.Name == name {
				refs = append(refs, resources.RuleSetReference{RuleSetID: rule.ID})
			}
		}
	}
	state, err := s.SaveStrategyGroup(resources.StrategyGroup{Name: "Routing", Kind: "rule", Policy: resources.PolicyTarget{Mode: "proxy"}, RuleSetReferences: refs[:1]})
	if err != nil {
		t.Fatal(err)
	}
	groupID := state.StrategyGroups[0].ID
	state, err = s.SaveStrategyGroup(resources.StrategyGroup{Name: "Selector", Kind: "selector", Type: "select", RuleSetReferences: refs[1:]})
	if err != nil {
		t.Fatal(err)
	}
	plan := resources.DefaultPlan()
	plan.StrategyGroupIDs = []string{groupID}
	for _, group := range state.StrategyGroups {
		if group.Name == "Selector" {
			plan.DefaultProxySelectorID = group.ID
		}
	}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Portable", Description: "Complete package", Fields: []profile.LocalConfigField{{Path: "/tcp-concurrent", ValueYAML: "true", Strategy: compose.StrategyReplace}}, ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	return local
}

func packageBytes(t *testing.T, p configtransfer.Package) []byte {
	t.Helper()
	data, err := configtransfer.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestConfigPackageRoundTripRemapsNamesAndPreservesAssociations(t *testing.T) {
	source, _, _ := fallbackService(t)
	original := portableFixture(t, source)
	if _, err := source.SaveStrategyGroup(resources.StrategyGroup{Name: "Unrelated", Kind: "selector", Type: "select"}); err != nil {
		t.Fatal(err)
	}
	data, filename, err := source.ExportLocalPackage(configtransfer.ConfigKind, original.ID)
	if err != nil || !strings.HasSuffix(filename, ".json") {
		t.Fatal("export", err)
	}
	p, err := configtransfer.Decode(data, configtransfer.ConfigKind)
	if err != nil || len(p.Config.StrategyGroups) != 2 || len(p.Config.RuleSets) != 2 {
		t.Fatal("incorrect graph closure", err)
	}

	target, _, subscriptions := fallbackService(t)
	current := portableFixture(t, target)
	if _, err := target.SetSubscriptionLocalConfig(subscriptions.SelectedSubscriptionID, current.ID); err != nil {
		t.Fatal(err)
	}
	beforeLibrary, _ := target.LocalConfigResources()
	// The package replaces fields, metadata and resource content, not identity.
	p.Config.Name, p.Config.Description = "Imported", "Restored backup"
	p.Config.Fields[0].ValueYAML = "false"
	p.Config.RuleSets[0].Description = "Imported rule description"
	data = packageBytes(t, p)
	preview, err := target.PreviewLocalPackage(configtransfer.ConfigKind, current.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preview.OverwrittenGroups, []string{"Routing", "Selector"}) || len(preview.OverwrittenRuleSets) != 2 || len(preview.AffectedSubscriptions) != 1 {
		t.Fatalf("incomplete preview: %+v", preview)
	}
	untouched, _ := target.LocalConfig(current.ID)
	if !reflect.DeepEqual(untouched, current) {
		t.Fatal("preview wrote target")
	}
	if err := target.ResolveLocalPackage(preview.Token, true); err != nil {
		t.Fatal(err)
	}
	after, _ := target.LocalConfig(current.ID)
	if after.ID != current.ID || after.CreatedAt != current.CreatedAt || after.Revision != current.Revision+1 || after.Name != "Imported" || after.Description != "Restored backup" || after.Fields[0].ValueYAML != "false" {
		t.Fatalf("incorrect target replacement: %+v", after)
	}
	if !reflect.DeepEqual(after.ResourcePlan, current.ResourcePlan) {
		t.Fatal("same-name resource IDs were not preserved")
	}
	library, _ := target.LocalConfigResources()
	if library.Revision != beforeLibrary.Revision+1 {
		t.Fatal("library did not commit once")
	}
	for i, rule := range library.RuleSets {
		if rule.ID != beforeLibrary.RuleSets[i].ID || !strings.Contains(rule.PayloadYAML, "# Keep this comment") {
			t.Fatal("rule ID or comments lost")
		}
	}
	detail, _ := target.Subscription(subscriptions.SelectedSubscriptionID)
	if detail.LocalConfigID != current.ID {
		t.Fatal("association lost")
	}
	if err := target.ResolveLocalPackage(preview.Token, true); err == nil {
		t.Fatal("confirmation replay accepted")
	}
	if target.runtimeManager.Status().PID != 0 {
		t.Fatal("import started the core")
	}
}

func TestConfigPackageAddsResourcesAndCancelDoesNotWrite(t *testing.T) {
	source, _, _ := fallbackService(t)
	local := portableFixture(t, source)
	data, _, _ := source.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	target, _, _ := fallbackService(t)
	current, err := target.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	beforeLibrary, _ := target.LocalConfigResources()
	preview, err := target.PreviewLocalPackage(configtransfer.ConfigKind, current.ID, data)
	if err != nil || len(preview.AddedGroups) != 2 || len(preview.AddedRuleSets) != 2 || len(preview.OverwrittenGroups) != 0 {
		t.Fatal("new resources preview", err)
	}
	if err := target.ResolveLocalPackage(preview.Token, false); err != nil {
		t.Fatal(err)
	}
	after, _ := target.LocalConfig(current.ID)
	library, _ := target.LocalConfigResources()
	if !reflect.DeepEqual(current, after) || !reflect.DeepEqual(beforeLibrary, library) {
		t.Fatal("cancel persisted a package")
	}
	preview, err = target.PreviewLocalPackage(configtransfer.ConfigKind, current.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.ResolveLocalPackage(preview.Token, true); err != nil {
		t.Fatal(err)
	}
	after, _ = target.LocalConfig(current.ID)
	if after.ResourcePlan.DefaultProxySelectorID == local.ResourcePlan.DefaultProxySelectorID {
		t.Fatal("import trusted foreign resource IDs")
	}
	if _, _, err := target.ExportLocalPackage(configtransfer.ConfigKind, current.ID); err != nil {
		t.Fatal("imported graph cannot be exported", err)
	}
}

func TestPackageRejectsStaleConfirmationAndBrokenCandidates(t *testing.T) {
	s, _, state := fallbackService(t)
	local := portableFixture(t, s)
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	data, _, _ := s.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	preview, err := s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveStrategyGroup(resources.StrategyGroup{Name: "Intervening", Kind: "selector", Type: "select"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveLocalPackage(preview.Token, true); err == nil {
		t.Fatal("stale library overwritten")
	}
	p, _ := configtransfer.Decode(data, configtransfer.ConfigKind)
	p.Config.Fields = append(p.Config.Fields, profile.LocalConfigField{Path: "/sub-rules", ValueYAML: "broken: ['MATCH,missing-selector']", Strategy: compose.StrategyReplace})
	before, _ := s.LocalConfigResources()
	if _, err := s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, packageBytes(t, p)); err == nil {
		t.Fatal("invalid candidate accepted")
	}
	after, _ := s.LocalConfigResources()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid candidate changed resources")
	}
	unchanged, _ := s.LocalConfig(local.ID)
	if !reflect.DeepEqual(unchanged, local) {
		t.Fatal("invalid candidate changed config")
	}
}

func TestPackageValidatesOtherConfigsAndInactiveSubscriptions(t *testing.T) {
	s, _, state := fallbackService(t)
	local := portableFixture(t, s)
	other, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Shared", ResourcePlan: local.ResourcePlan, Fields: []profile.LocalConfigField{{Path: "/sub-rules", ValueYAML: "shared: ['MATCH,Selector']", Strategy: compose.StrategyReplace}}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Inactive", SourceURL: "https://example.test/inactive"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range second.Subscriptions {
		if item.ID != state.SelectedSubscriptionID {
			if _, err := s.SetSubscriptionLocalConfig(item.ID, other.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.SelectSubscription(state.SelectedSubscriptionID); err != nil {
		t.Fatal(err)
	}
	data, _, _ := s.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	p, _ := configtransfer.Decode(data, configtransfer.ConfigKind)
	for i := range p.Config.StrategyGroups {
		if p.Config.StrategyGroups[i].Name == "Selector" {
			p.Config.StrategyGroups[i].Emoji = "🎯"
		}
	}
	before, _ := s.LocalConfigResources()
	if _, err := s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, packageBytes(t, p)); err == nil || !strings.Contains(err.Error(), "Inactive") {
		t.Fatal("inactive shared-resource subscriber was not checked", err)
	}
	after, _ := s.LocalConfigResources()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("shared-resource failure changed library")
	}
}

func TestScriptPackagePreservesIdentityAndRejectsRuntimeErrors(t *testing.T) {
	s, _, state := fallbackService(t)
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Script", Contents: "function main(c) { return c; }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalScript(state.SelectedSubscriptionID, script.ID); err != nil {
		t.Fatal(err)
	}
	data, _, err := s.ExportLocalPackage(configtransfer.ScriptKind, script.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := configtransfer.Decode(data, configtransfer.ScriptKind)
	p.Script.Name, p.Script.Contents = "Imported script", "// keep comment\nfunction main(c) { c['tcp-concurrent'] = true; return c; }"
	preview, err := s.PreviewLocalPackage(configtransfer.ScriptKind, script.ID, packageBytes(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveLocalPackage(preview.Token, true); err != nil {
		t.Fatal(err)
	}
	after, _ := s.LocalScript(script.ID)
	if after.Revision != script.Revision+1 || after.Name != p.Script.Name || !strings.Contains(after.Contents, "// keep comment") {
		t.Fatal("script not fully imported")
	}
	detail, _ := s.Subscription(state.SelectedSubscriptionID)
	if detail.LocalScriptID != script.ID {
		t.Fatal("script association lost")
	}
	p.Script.Contents = "function main(c) { throw new Error('invalid import'); }"
	if _, err := s.PreviewLocalPackage(configtransfer.ScriptKind, script.ID, packageBytes(t, p)); err == nil {
		t.Fatal("broken script imported")
	}
	unchanged, _ := s.LocalScript(script.ID)
	if !reflect.DeepEqual(after, unchanged) {
		t.Fatal("script rejection changed source")
	}
}

func TestConfigPackageLibraryWriteFailureRollsBackTarget(t *testing.T) {
	s, _, _ := fallbackService(t)
	local := portableFixture(t, s)
	data, _, _ := s.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	p, _ := configtransfer.Decode(data, configtransfer.ConfigKind)
	p.Config.Name = "Must not persist"
	preview, err := s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, packageBytes(t, p))
	if err != nil {
		t.Fatal(err)
	}
	// Exceed the library's persistence limit only at commit, after the config
	// has been staged. This exercises the two-store rollback without touching
	// real user data or relying on platform-specific permission failures.
	base := s.pendingPackage.resources.State.RuleSets[0]
	base.PayloadYAML = "#" + strings.Repeat("x", 128<<10) + "\npayload:\n  - DOMAIN,example.test\n"
	for i := 0; i < 70; i++ {
		rule := base
		rule.ID, rule.Name = fmt.Sprintf("%032x", i+1), fmt.Sprintf("Extra %d", i+1)
		s.pendingPackage.resources.State.RuleSets = append(s.pendingPackage.resources.State.RuleSets, rule)
	}
	before, _ := os.ReadFile(filepath.Join(s.localConfigs.Directory(), local.ID, "manifest.json"))
	if err := s.ResolveLocalPackage(preview.Token, true); err == nil || !strings.Contains(err.Error(), "library is too large") {
		t.Fatal("expected library persistence failure", err)
	}
	after, _ := os.ReadFile(filepath.Join(s.localConfigs.Directory(), local.ID, "manifest.json"))
	if string(before) != string(after) {
		t.Fatal("library failure changed target file")
	}
}

func TestPackageConfirmationRevalidatesNewAssociations(t *testing.T) {
	s, _, subscriptions := fallbackService(t)
	local := portableFixture(t, s)
	data, _, _ := s.ExportLocalPackage(configtransfer.ConfigKind, local.ID)
	p, _ := configtransfer.Decode(data, configtransfer.ConfigKind)
	p.Config.Fields = append(p.Config.Fields, profile.LocalConfigField{Path: "/sub-rules", ValueYAML: "new: ['MATCH,missing-selector']", Strategy: compose.StrategyReplace})
	preview, err := s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, packageBytes(t, p))
	if err != nil {
		t.Fatal("unassociated draft preview", err)
	}
	if _, err := s.SetSubscriptionLocalConfig(subscriptions.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := s.LocalConfigResources()
	if err := s.ResolveLocalPackage(preview.Token, true); err == nil {
		t.Fatal("confirmation ignored newly associated subscription")
	}
	after, _ := s.LocalConfig(local.ID)
	library, _ := s.LocalConfigResources()
	if !reflect.DeepEqual(after, local) || !reflect.DeepEqual(before, library) {
		t.Fatal("rejected confirmation changed data")
	}
	preview, err = s.PreviewLocalPackage(configtransfer.ConfigKind, local.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	s.pendingPackage.expires = time.Now().Add(-time.Second)
	if err := s.ResolveLocalPackage(preview.Token, true); err == nil {
		t.Fatal("expired package confirmation accepted")
	}
}

func TestScriptPackageConfirmationProtectsInterveningEdits(t *testing.T) {
	s, _, _ := fallbackService(t)
	current, err := s.SaveLocalScript(localscript.SaveInput{Name: "Before", Contents: "function main(c) { return c; }"})
	if err != nil {
		t.Fatal(err)
	}
	data, _, _ := s.ExportLocalPackage(configtransfer.ScriptKind, current.ID)
	preview, err := s.PreviewLocalPackage(configtransfer.ScriptKind, current.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	newer, err := s.SaveLocalScript(localscript.SaveInput{ID: current.ID, Name: "Edited later", Contents: current.Contents})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResolveLocalPackage(preview.Token, true); err == nil {
		t.Fatal("stale script confirmation accepted")
	}
	after, _ := s.LocalScript(current.ID)
	if !reflect.DeepEqual(newer, after) {
		t.Fatal("confirmation overwrote later edit")
	}
}
