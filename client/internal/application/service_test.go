package application

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"jeemi/internal/config/compose"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/core"
	"jeemi/internal/localscript"
	"jeemi/internal/mihomo/delaycache"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/settings"
	"jeemi/internal/subscription"
)

func TestBootstrapStateReportsFrameworkWithoutPretendingCoreIsInstalled(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	state := service.BootstrapState()

	if state.App.Name != "Jeemi" {
		t.Fatalf("unexpected app name: %q", state.App.Name)
	}
	if state.Runtime.Core.State != core.StateNotInstalled {
		t.Fatalf("unexpected initial core state: %q", state.Runtime.Core.State)
	}
	if state.Runtime.SystemProxy || state.Runtime.TunEnabled {
		t.Fatal("system proxy and TUN must be disabled in the initial framework")
	}
	if _, err := time.Parse(time.RFC3339Nano, state.Runtime.ClientStartedAt); err != nil {
		t.Fatalf("client start time is not RFC3339: %q", state.Runtime.ClientStartedAt)
	}
	if refreshed := service.RuntimeStatus(); refreshed.ClientStartedAt != state.Runtime.ClientStartedAt {
		t.Fatalf("client start time changed across status refresh: %q != %q", refreshed.ClientStartedAt, state.Runtime.ClientStartedAt)
	}
}

func TestServiceExposesLocalConfigurationWithoutStartingMihomo(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	created, err := service.SaveLocalConfig(profile.SaveLocalConfigInput{
		Name: "Test local configuration",
		Fields: []profile.LocalConfigField{{
			Path: "/dns/use-system-hosts", ValueYAML: "true", Strategy: compose.StrategyReplace,
		}},
	})
	if err != nil {
		t.Fatalf("SaveLocalConfig() error = %v", err)
	}
	state, err := service.LocalConfigState()
	if err != nil {
		t.Fatalf("LocalConfigState() error = %v", err)
	}
	if len(state.Configs) != 1 || state.Configs[0].ID != created.ID {
		t.Fatalf("unexpected configuration state: %+v", state)
	}
	if service.RuntimeStatus().Core.State != core.StateNotInstalled {
		t.Fatal("saving a local configuration changed the mihomo runtime state")
	}
}

func TestServiceKeepsSubscriptionAssociationReferentiallySafe(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	local, err := service.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Reusable local configuration"})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	if err := os.WriteFile(source, []byte("mode: rule\nproxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionID := state.Subscriptions[0].ID
	if _, err := service.SetSubscriptionLocalConfig(subscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteLocalConfig(local.ID); err == nil {
		t.Fatal("DeleteLocalConfig() removed a referenced local configuration")
	}
	if _, err := service.SetSubscriptionLocalConfig(subscriptionID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteLocalConfig(local.ID); err != nil {
		t.Fatalf("DeleteLocalConfig() failed after association removal: %v", err)
	}
	if service.RuntimeStatus().Core.State != core.StateNotInstalled {
		t.Fatal("subscription management changed the mihomo runtime state")
	}
}

func TestServiceValidatesLocalScriptBeforeAssociationAndReferencedSave(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	if err := os.WriteFile(source, []byte("mode: rule\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionID := state.Subscriptions[0].ID

	invalid, err := service.SaveLocalScript(localscript.SaveInput{
		Name: "Invalid output", Contents: "function main() { return []; }",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetSubscriptionLocalScript(subscriptionID, invalid.ID); err == nil {
		t.Fatal("SetSubscriptionLocalScript() persisted a script whose output is not a configuration mapping")
	}

	valid, err := service.SaveLocalScript(localscript.SaveInput{
		Name: "Valid", Contents: "function main(config) { config.profile = {'store-selected': true}; return config; }",
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err = service.SetSubscriptionLocalScript(subscriptionID, valid.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Subscriptions[0].LocalScriptID != valid.ID || state.Projection == nil || state.Projection.LocalScriptRevision != valid.Revision {
		t.Fatalf("local script association was not projected: %+v", state)
	}
	if _, err := service.DeleteLocalScript(valid.ID); err == nil {
		t.Fatal("DeleteLocalScript() removed a referenced script")
	}
	if _, err := service.SaveLocalScript(localscript.SaveInput{
		ID: valid.ID, Name: valid.Name, Contents: "function main() { return []; }",
	}); err == nil {
		t.Fatal("SaveLocalScript() replaced a referenced script with invalid output")
	}
	reloaded, err := service.LocalScript(valid.ID)
	if err != nil || reloaded.Revision != valid.Revision {
		t.Fatalf("failed referenced save changed the stored script: %+v err=%v", reloaded, err)
	}
}

func TestServiceFormatsLocalScriptTestUnicodeForDisplay(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	if err := os.WriteFile(source, []byte("mode: rule\nproxies: []\nproxy-groups: []\nrules:\n  - MATCH,DIRECT\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.TestLocalScript(localscript.TestInput{
		SaveInput: localscript.SaveInput{
			Name: "Unicode preview",
			Contents: `function main(config) {
  config.preview = "\uD83C\uDDEF\uD83C\uDDF5 Jeemi \uD83D\uDE80";
  config.literal = "\\U0001F680";
  return config;
}`,
		},
		SubscriptionID: state.Subscriptions[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Contents, "🇯🇵 Jeemi 🚀") {
		t.Fatalf("local script test did not restore Unicode for display:\n%s", result.Contents)
	}
	if strings.Contains(result.Contents, `\U0001F1EF`) {
		t.Fatalf("local script test still contains an escaped flag:\n%s", result.Contents)
	}
	if !strings.Contains(result.Contents, `literal: \U0001F680`) {
		t.Fatalf("local script test changed a literal Unicode escape:\n%s", result.Contents)
	}
}

func TestServiceReportsOnlyManagedDomainDirectories(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}

	directories := service.ManagedStorageDirectories()
	if directories.Subscriptions != filepath.Join(dataDirectory, "subscriptions") {
		t.Fatalf("unexpected subscription directory: %q", directories.Subscriptions)
	}
	if directories.LocalConfigs != filepath.Join(dataDirectory, "local-configs") {
		t.Fatalf("unexpected local configuration directory: %q", directories.LocalConfigs)
	}
	if directories.LocalScripts != filepath.Join(dataDirectory, "local-scripts") {
		t.Fatalf("unexpected local script directory: %q", directories.LocalScripts)
	}
}

func TestServicePersistsCompleteSubscriptionPreferences(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.SaveSubscriptionPreferences(subscription.PreferencesInput{
		SelectorDensity:      settings.SelectorDensitySmall,
		DelayTestConcurrency: 16,
		ConnectionResetMode:  settings.ConnectionResetAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.SelectorDensity != settings.SelectorDensitySmall || saved.DelayTestConcurrency != 16 || saved.ConnectionResetMode != settings.ConnectionResetAll || saved.SelectorSortMode != settings.DefaultSelectorSortMode || saved.SelectorViewMode != settings.DefaultSelectorViewMode {
		t.Fatalf("saved subscription preferences = %+v", saved)
	}
	saved, err = service.SaveSubscriptionSelectorDisplayPreferences(subscription.SelectorDisplayPreferencesInput{
		SelectorSortMode: settings.SelectorSortName,
		SelectorViewMode: settings.SelectorViewTabs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.SelectorDensity != settings.SelectorDensitySmall || saved.DelayTestConcurrency != 16 || saved.ConnectionResetMode != settings.ConnectionResetAll || saved.SelectorSortMode != settings.SelectorSortName || saved.SelectorViewMode != settings.SelectorViewTabs {
		t.Fatalf("saved selector display preferences = %+v", saved)
	}
	state, err := service.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Preferences != saved {
		t.Fatalf("subscription state lost preferences: %+v", state.Preferences)
	}
}

func TestServicePersistsRuntimePreferencesWithoutChangingLiveStatus(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	preferences := runtimeconfig.DefaultPreferences()
	preferences.OutboundMode = runtimeconfig.OutboundModeDirect
	preferences.ProxyMode = runtimeconfig.ProxyModeTUN
	preferences.ListenerType = runtimeconfig.ListenerTypeMixed
	preferences.ListenPort = 7897
	preferences.AllowLAN = true
	preferences.TUNStack = runtimeconfig.TUNStackSystem
	saved, err := service.SaveRuntimePreferences(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, preferences) {
		t.Fatalf("SaveRuntimePreferences() = %+v", saved)
	}
	reloaded, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reloaded.RuntimePreferences()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, preferences) {
		t.Fatalf("RuntimePreferences() = %+v", loaded)
	}
	status := reloaded.RuntimeStatus()
	if status.ProxyMode != "off" || status.SystemProxy || status.TunEnabled {
		t.Fatalf("saved startup preferences were reported as live state: %+v", status)
	}
}

func TestServiceExposesRuntimeDefaultsAndValidatesOnlyAllowListedYAML(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	defaults := service.DefaultLANBypassRules()
	if !reflect.DeepEqual(defaults, runtimeconfig.DefaultLANBypassRules()) {
		t.Fatalf("DefaultLANBypassRules() = %+v", defaults)
	}
	valid := service.ValidateRuntimeYAMLFragment(runtimeconfig.YAMLFragmentInput{
		Field:    runtimeconfig.YAMLFieldLANBypassRules,
		Contents: "[]",
	})
	if !valid.Valid || len(valid.Values) != 0 {
		t.Fatalf("empty LAN rule YAML was rejected: %+v", valid)
	}
	invalid := service.ValidateRuntimeYAMLFragment(runtimeconfig.YAMLFragmentInput{
		Field:    "/arbitrary/path",
		Contents: "value",
	})
	if invalid.Valid || invalid.Issue == nil {
		t.Fatalf("arbitrary YAML field was accepted: %+v", invalid)
	}
}

func TestServiceBuildsStaticSelectorProjectionFromAssociatedLocalConfiguration(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	resourceState, err := service.SaveRuleSet(configresources.RuleSet{
		Name: "local", SourceType: "http", Behavior: "domain", Format: "yaml", URL: "https://example.invalid/rules.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	resourceState, err = service.SaveStrategyGroup(configresources.StrategyGroup{
		Name: "Direct", Kind: configresources.StrategyGroupKindRule,
		Policy:            configresources.PolicyTarget{Mode: configresources.PolicyDirect},
		RuleSetReferences: []configresources.RuleSetReference{{RuleSetID: resourceState.RuleSets[0].ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{resourceState.StrategyGroups[0].ID}
	local, err := service.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Rules", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	contents := []byte("proxies:\n  - name: node-a\n    type: ss\nproxy-groups:\n  - name: Main\n    type: select\n    hidden: true\n    proxies: [node-a, DIRECT]\n")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	state, err = service.SetSubscriptionLocalConfig(state.Subscriptions[0].ID, local.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection == nil || state.Projection.Status != "ready" || len(state.Projection.Selectors) != 1 || len(state.Projection.RuleProviders) != 1 {
		t.Fatalf("unexpected static projection: %+v", state.Projection)
	}
	if !state.Projection.Selectors[0].Hidden {
		t.Fatalf("selector hidden flag was lost at the application boundary: %+v", state.Projection.Selectors[0])
	}
	if state.Projection.CoreValidationStatus != "not_run" || service.RuntimeStatus().Core.State != core.StateNotInstalled {
		t.Fatal("static projection pretended to validate or start mihomo")
	}
}

func TestServiceResolvesReusableGroupsAndRuleSetsIntoSubscriptionProjection(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	resourceState, err := service.SaveRuleSet(configresources.RuleSet{
		Name: "本地域名", SourceType: "inline", Behavior: "domain", PayloadYAML: "payload:\n  - +.example.com\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	ruleSetID := resourceState.RuleSets[0].ID
	resourceState, err = service.SaveStrategyGroup(configresources.StrategyGroup{
		Name: "德国节点", Kind: configresources.StrategyGroupKindSelector,
		RuleOutput: configresources.RuleOutputRuleSet, Type: "select",
		Filter: configresources.NodeFilter{NamePatterns: []string{"🇩🇪"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	selectorID := resourceState.StrategyGroups[0].ID
	resourceState, err = service.SaveStrategyGroup(configresources.StrategyGroup{
		Name: "德国域名规则", Kind: configresources.StrategyGroupKindRule,
		RuleOutput:        configresources.RuleOutputRuleSet,
		RuleSetReferences: []configresources.RuleSetReference{{RuleSetID: ruleSetID}},
		Policy:            configresources.PolicyTarget{Mode: configresources.PolicyProxy},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ruleGroupID string
	for _, group := range resourceState.StrategyGroups {
		if group.Name == "德国域名规则" {
			ruleGroupID = group.ID
		}
	}
	if ruleGroupID == "" {
		t.Fatal("saved rule group was not returned")
	}
	local, err := service.SaveLocalConfig(profile.SaveLocalConfigInput{
		Name: "可复用资源引用",
		ResourcePlan: configresources.Plan{
			StrategyGroupIDs:       []string{ruleGroupID},
			RuleStrategy:           compose.StrategyAppendBeforeTerminal,
			DefaultProxySelectorID: selectorID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription-resources.yaml")
	contents := []byte("proxies:\n  - name: 🇩🇪Base.NB.de\n    type: vless\n  - name: 🇯🇵Base.NB.jp\n    type: vless\nproxy-groups:\n  - name: Existing\n    type: select\n    proxies: [DIRECT]\nrules:\n  - MATCH,Existing\n")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	state, err = service.SetSubscriptionLocalConfig(state.Subscriptions[0].ID, local.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection == nil || state.Projection.Status != projectionReady || len(state.Projection.Selectors) != 2 || len(state.Projection.RuleProviders) != 1 {
		t.Fatalf("reusable resources were not projected: %+v", state.Projection)
	}
	group := state.Projection.Selectors[1]
	if group.Name != "德国节点" || len(group.Members) != 1 || !strings.Contains(group.Members[0].Name, "Base.NB.de") {
		t.Fatalf("strategy group filter was not materialized: %+v", group)
	}
	if !group.ReferencedByRules || !state.Projection.Selectors[0].ReferencedByRules {
		t.Fatalf("selector rule usage was not projected: %+v", state.Projection.Selectors)
	}
	if len(state.Projection.Proxies) != 2 {
		t.Fatalf("global proxy members were not projected: %+v", state.Projection.Proxies)
	}
	if _, err := service.DeleteStrategyGroup(selectorID); err == nil {
		t.Fatal("referenced strategy group was deleted")
	}
	if _, err := service.DeleteRuleSet(ruleSetID); err == nil {
		t.Fatal("referenced rule set was deleted")
	}
}

func TestServiceAppliesPerSubscriptionRuleProviderSwitchesToProjectionAndResolvedConfig(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "providers.yaml")
	contents := []byte(`rule-providers:
  ads:
    type: http
    behavior: domain
    format: yaml
    url: https://example.invalid/ads.yaml
  private:
    type: http
    behavior: domain
    format: yaml
    url: https://example.invalid/private.yaml
rules:
  - RULE-SET,ads,REJECT
  - RULE-SET,private,DIRECT
  - MATCH,DIRECT
`)
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionID := state.SelectedSubscriptionID
	state, err = service.SetSubscriptionRuleProviderEnabled(subscriptionID, "ads", false)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection == nil || state.Projection.RuleProviderOverrideRevision != 1 || len(state.Projection.RuleProviders) != 2 || state.Projection.Summary.RuleProviderCount != 1 {
		t.Fatalf("unexpected provider projection: %+v", state.Projection)
	}
	providerEnabled := map[string]bool{}
	for _, provider := range state.Projection.RuleProviders {
		providerEnabled[provider.Name] = provider.Enabled
	}
	if providerEnabled["ads"] || !providerEnabled["private"] {
		t.Fatalf("provider switches were not projected: %+v", providerEnabled)
	}
	resolvedPath := filepath.Join(dataDirectory, "runtime", "mihomo", "resolved", subscriptionID, "current.yaml")
	resolved, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(resolved, []byte("RULE-SET,ads")) || bytes.Contains(resolved, []byte("  ads:")) {
		t.Fatalf("disabled provider remains in resolved configuration:\n%s", resolved)
	}
	if !bytes.Contains(resolved, []byte("RULE-SET,private")) {
		t.Fatalf("enabled provider was removed:\n%s", resolved)
	}
	status := service.RuntimeStatus().Configuration
	if status.RuleProviderOverrideRevision != 1 {
		t.Fatalf("runtime status lost the provider override revision: %+v", status)
	}
	state, err = service.SetSubscriptionRuleProviderEnabled(subscriptionID, "ads", true)
	if err != nil || state.Projection == nil || state.Projection.Summary.RuleProviderCount != 2 {
		t.Fatalf("provider could not be re-enabled: projection=%+v err=%v", state.Projection, err)
	}
}

func TestServicePersistsCurrentSubscriptionWithoutTreatingItAsRuntime(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for index, name := range []string{"first.yaml", "second.yaml"} {
		path := filepath.Join(t.TempDir(), name)
		contents := []byte(fmt.Sprintf("mode: rule\nport: %d\n", 7800+index))
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		state, err := service.ImportSubscriptionFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range state.Subscriptions {
			found := false
			for _, existing := range ids {
				if item.ID == existing {
					found = true
				}
			}
			if !found {
				ids = append(ids, item.ID)
			}
		}
	}
	selectedID := ids[len(ids)-1]
	if _, err := service.SelectSubscription(selectedID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := reloaded.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	if state.SelectedSubscriptionID != selectedID || state.Projection == nil || state.Projection.SubscriptionID != selectedID {
		t.Fatalf("selected subscription did not survive reload: %+v", state)
	}
	if reloaded.RuntimeStatus().Core.State != core.StateNotInstalled {
		t.Fatal("selecting a subscription changed runtime state")
	}
}

func TestServiceAutomaticallyMaintainsSafeResolvedRuntimeSnapshot(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "subscription.yaml")
	contents := []byte("external-controller: 0.0.0.0:9090\nsecret: source-secret\nmode: rule\nport: 8080\nproxies: []\n")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := service.ImportSubscriptionFile(source)
	if err != nil {
		t.Fatal(err)
	}
	resolvedPath := filepath.Join(dataDirectory, "runtime", "mihomo", "resolved", state.SelectedSubscriptionID, "current.yaml")
	resolvedContents, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(resolvedContents, []byte("source-secret")) || bytes.Contains(resolvedContents, []byte("external-controller")) {
		t.Fatalf("resolved snapshot retained subscription control fields:\n%s", resolvedContents)
	}
	if !bytes.Contains(resolvedContents, []byte("mixed-port: 7890")) {
		t.Fatalf("default runtime listener was not injected:\n%s", resolvedContents)
	}

	preferences := runtimeconfig.DefaultPreferences()
	preferences.OutboundMode = runtimeconfig.OutboundModeDirect
	preferences.ListenPort = 7897
	if _, err := service.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(resolvedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(updated, []byte("mode: direct")) || !bytes.Contains(updated, []byte("mixed-port: 7897")) {
		t.Fatalf("automatic runtime preference reconciliation did not update snapshot:\n%s", updated)
	}
	status := service.RuntimeStatus().Configuration
	if status.State != RuntimeConfigurationReady || status.DesiredFingerprint == "" || status.CoreValidated {
		t.Fatalf("unexpected stopped-runtime configuration status: %+v", status)
	}

	view, err := service.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	if view.Source != runtimeConfigurationViewResolved || view.SubscriptionID != state.SelectedSubscriptionID {
		t.Fatalf("unexpected stopped runtime configuration view: %+v", view)
	}
	if strings.Contains(view.Contents, "source-secret") || strings.Contains(view.Contents, "external-controller") || strings.Contains(view.Contents, "secret:") {
		t.Fatalf("runtime configuration view exposed controller fields:\n%s", view.Contents)
	}
}

func TestInvalidRuntimePreferencesAreNotPersistedAndAreReportedOnHomeStatus(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	invalid := runtimeconfig.DefaultPreferences()
	invalid.ListenPort = 0
	if _, err := service.SaveRuntimePreferences(invalid); err == nil {
		t.Fatal("SaveRuntimePreferences() accepted an invalid port")
	}
	loaded, err := service.RuntimePreferences()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ListenPort != runtimeconfig.DefaultListenPort {
		t.Fatalf("invalid preferences replaced persisted settings: %+v", loaded)
	}
	status := service.RuntimeStatus().Configuration
	if status.State != RuntimeConfigurationFailed || status.LastError == nil || status.LastError.Code != "runtime_preferences_invalid" {
		t.Fatalf("invalid preference failure was not exposed through runtime status: %+v", status)
	}
}

func TestStopProxyClearsActiveFingerprintWithoutHidingConfigurationFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jeemi_data")
	service, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	// A configuration-status test must never contact the installed helper or
	// restore the developer's real proxy, even while the runtime is stopped.
	driver := &authorizationDriver{}
	service.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{DataDirectory: root, Driver: driver, SystemProxy: fallbackTestSystemProxy{}})
	if err != nil {
		t.Fatal(err)
	}
	service.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.State = RuntimeConfigurationFailed
		status.DesiredFingerprint = strings.Repeat("a", 64)
		status.AppliedFingerprint = strings.Repeat("b", 64)
		status.LastError = &mihomoruntime.Failure{
			Code: "configuration_validation_failed", Phase: "validate", Message: "invalid configuration",
		}
	})

	stopped, err := service.StopProxy()
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Configuration.State != RuntimeConfigurationFailed || stopped.Configuration.LastError == nil {
		t.Fatalf("stop hid the pending configuration failure: %+v", stopped.Configuration)
	}
	if stopped.Configuration.AppliedFingerprint != "" {
		t.Fatalf("stopped runtime retained an active fingerprint: %+v", stopped.Configuration)
	}
	if driver.starts.Load() != 0 {
		t.Fatal("stopping implicitly started a core")
	}
}

func TestServicePersistsProxyDelayCacheWithoutChangingRuntime(t *testing.T) {
	service, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	scope := delaycache.Scope{
		SubscriptionID:       strings.Repeat("a", 32),
		SubscriptionRevision: strings.Repeat("b", 64),
	}
	if _, err := service.SaveProxyDelayCache(delaycache.Update{
		Scope: scope,
		Results: []delaycache.UpdateEntry{{
			Name: "Node A", Status: delaycache.StatusSuccess,
			Delay: 88, Source: delaycache.SourceManual,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := service.ProxyDelayCache(scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Results) != 1 || loaded.Results[0].Delay != 88 {
		t.Fatalf("unexpected proxy delay cache: %+v", loaded)
	}
	if service.RuntimeStatus().Mihomo.State != mihomoruntime.StateStopped {
		t.Fatal("saving delay results changed the mihomo runtime state")
	}
}
