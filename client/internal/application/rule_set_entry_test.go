package application

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configresources "jeemi/internal/config/resources"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
)

func TestConnectionRuleEntryValidatesBeforeReplacingReferencedRuleSet(t *testing.T) {
	s, healthyDriver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	subscriptions, err := s.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	library, err := s.SaveRuleSetEntry(configresources.RuleSetEntryInput{Name: "Connection rules", MatchType: "domain", Value: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	ruleSetID := library.RuleSets[0].ID
	library, err = s.SaveStrategyGroup(configresources.StrategyGroup{
		Name: "Routing", Kind: "rule", Policy: configresources.PolicyTarget{Mode: "direct"},
		RuleOutput:        configresources.RuleOutputInline,
		RuleSetReferences: []configresources.RuleSetReference{{RuleSetID: ruleSetID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{library.StrategyGroups[0].ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(subscriptions.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	before, err := s.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	libraryBytes, err := os.ReadFile(filepath.Join(library.Directory, "library.json"))
	if err != nil {
		t.Fatal(err)
	}

	// The imported binary is synthetic and both drivers are test doubles.
	driver := &rejectingCandidateDriver{}
	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{
		DataDirectory: filepath.Dir(s.localConfigs.Directory()), Driver: driver, SystemProxy: fallbackTestSystemProxy{},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := configresources.RuleSetEntryInput{
		RuleSetID: ruleSetID, ExpectedRevision: library.Revision, MatchType: "processName", Value: "WeChat.exe",
	}
	preview, err := s.PreviewRuleSetEntry(input)
	if err != nil || !preview.Valid || driver.validations != 0 {
		t.Fatalf("preview should check syntax without invoking the core: %+v, %v", preview, err)
	}
	if _, err := s.SaveRuleSetEntry(input); err == nil || !strings.Contains(err.Error(), "fixture core rejected candidate") {
		t.Fatalf("candidate rejection ignored: %v", err)
	}
	if driver.validations == 0 || driver.starts.Load() != 0 || healthyDriver.starts.Load() != 0 {
		t.Fatal("saving must validate the candidate without starting the core")
	}
	afterBytes, err := os.ReadFile(filepath.Join(library.Directory, "library.json"))
	if err != nil || string(afterBytes) != string(libraryBytes) {
		t.Fatal("rejection changed the library", err)
	}
	after, err := s.RuntimeConfigurationText()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("rejection changed resolved configuration", err)
	}
	if _, err := s.LocalConfigResources(); err != nil {
		t.Fatal("rejection blocked the resource editor", err)
	}
	if _, err := s.SubscriptionState(); err != nil {
		t.Fatal("rejection blocked the subscription page", err)
	}

	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{
		DataDirectory: filepath.Dir(s.localConfigs.Directory()), Driver: healthyDriver, SystemProxy: fallbackTestSystemProxy{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveRuleSetEntry(input); err != nil {
		t.Fatal("corrected candidate could not be saved", err)
	}
	after, err = s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(after.Contents, "PROCESS-NAME,WeChat.exe,DIRECT") {
		t.Fatalf("successful append was not recomposed into resolved configuration: %+v, %v", after, err)
	}
	if healthyDriver.starts.Load() != 0 {
		t.Fatal("successful edit started the core")
	}
}
