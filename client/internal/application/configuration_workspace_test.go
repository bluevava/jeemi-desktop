package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/config/resources"
	"jeemi/internal/configworkspace"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
	"jeemi/internal/settings"
)

func TestProductionWorkspacePreservesAssociationsAndRepeatedLegacySwitches(t *testing.T) {
	legacy, root, state := chainService(t)
	id := state.SelectedSubscriptionID
	library, chainID := manualChain(t, legacy)
	if _, err := legacy.SetSubscriptionChainProxyGroups(id, []string{chainID}, 0, library.Revision); err != nil {
		t.Fatal(err)
	}
	script, err := legacy.SaveLocalScript(localscript.SaveInput{Name: "script", Contents: "function main(config) { return config; }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.SetSubscriptionLocalScript(id, script.ID); err != nil {
		t.Fatal(err)
	}
	rules, err := legacy.configResources.SaveRuleSet(resources.RuleSet{Name: "rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload:\n  - DOMAIN,example.test\n"})
	if err != nil {
		t.Fatal(err)
	}
	rules, err = legacy.configResources.SaveStrategyGroup(resources.StrategyGroup{Name: "selector", Kind: "selector", Type: "select", RuleSetReferences: []resources.RuleSetReference{{RuleSetID: rules.RuleSets[0].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	plan := resources.DefaultPlan()
	plan.StrategyGroupIDs = []string{rules.StrategyGroups[0].ID}
	local, err := legacy.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "local", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	before := workspaceConfigurationBytes(t, root)
	workspace, err := PrepareConfigurationWorkspace(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path, raw := range before {
		copied, err := os.ReadFile(filepath.Join(workspace, path))
		if err != nil || !bytes.Equal(raw, copied) {
			t.Fatalf("migration changed %s: %v", path, err)
		}
	}
	newer, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	if newer.subscriptions.Directory() == legacy.subscriptions.Directory() || newer.localConfigs.Directory() == legacy.localConfigs.Directory() {
		t.Fatal("configuration still shares the legacy root")
	}
	if _, err := newer.buildRuntimeCandidate(); err != nil {
		t.Fatal("copied graph cannot compose", err)
	}
	loaded, err := newer.Subscription(id)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LocalScriptID != script.ID || len(loaded.ChainProxyGroupIDs) != 1 || loaded.ChainProxyGroupIDs[0] != chainID {
		t.Fatal("migration lost subscription associations")
	}
	if config, err := newer.LocalConfig(local.ID); err != nil || config.ResourcePlan.StrategyGroupIDs[0] != rules.StrategyGroups[0].ID {
		t.Fatal("migration lost resource identities", err)
	}
	if _, err := newer.settingsStore.SetMihomoVersion("new-core"); err != nil {
		t.Fatal(err)
	}
	if _, err := newer.SaveLocalScript(localscript.SaveInput{ID: script.ID, Name: "new script", Contents: script.Contents + "\n// newer edit\n"}); err != nil {
		t.Fatal(err)
	}
	for path, raw := range before {
		actual, _ := os.ReadFile(filepath.Join(root, path))
		if !bytes.Equal(raw, actual) {
			t.Fatal("new version modified legacy configuration", path)
		}
	}
	for i := 0; i < 3; i++ {
		older, err := newTestService(root)
		if err != nil {
			t.Fatal("legacy root cannot reopen", err)
		}
		if _, err := older.settingsStore.SetMihomoVersion("old-core"); err != nil {
			t.Fatal(err)
		}
		if original, err := older.localScripts.Get(script.ID); err != nil || original.Name != script.Name {
			t.Fatal("older version received newer edits", err)
		}
		if selected, err := PrepareConfigurationWorkspace(context.Background(), root); err != nil || selected != workspace {
			t.Fatal(err)
		}
		newer, err = NewService(root)
		if err != nil {
			t.Fatal(err)
		}
		prefs, err := newer.settingsStore.Load()
		if err != nil || prefs.Mihomo.SelectedVersion != "new-core" {
			t.Fatal("legacy edit overwrote new preference", err)
		}
		if current, err := newer.localScripts.Get(script.ID); err != nil || current.Name != "new script" {
			t.Fatal("legacy switch overwrote new resource", err)
		}
	}
	if _, err := legacy.DeleteSubscription(id); err != nil {
		t.Fatal(err)
	}
	if _, err := newer.SubscriptionText(id); err != nil {
		t.Fatal("legacy deletion removed new subscription content", err)
	}
}

func TestMigrationRejectsBrokenReferenceAndCanRetryWithoutChangingOriginal(t *testing.T) {
	legacy, root, state := chainService(t)
	id := state.SelectedSubscriptionID
	path := filepath.Join(legacy.subscriptions.Directory(), id, "metadata.json")
	original, _ := os.ReadFile(path)
	var fields map[string]json.RawMessage
	json.Unmarshal(original, &fields)
	fields["localScriptId"] = json.RawMessage(`"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`)
	broken, _ := json.Marshal(fields)
	os.WriteFile(path, broken, 0600)
	if _, err := PrepareConfigurationWorkspace(context.Background(), root); err == nil {
		t.Fatal("migrated broken association")
	}
	target, _ := configworkspace.Directory(root, 1)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("invalid workspace published")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(broken, unchanged) {
		t.Fatal("failed migration changed legacy metadata")
	}
	os.WriteFile(path, original, 0600)
	if _, err := PrepareConfigurationWorkspace(context.Background(), root); err != nil {
		t.Fatal("repaired source cannot retry", err)
	}
	if _, err := NewService(root); err != nil {
		t.Fatal(err)
	}
}

func TestServiceRequiresPreparedWorkspaceAndKeepsCoreRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := NewService(root); err == nil {
		t.Fatal("service silently used an unprepared workspace")
	}
	if _, err := PrepareConfigurationWorkspace(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.settingsStore.SetMihomoVersion("v1.19.34"); err != nil {
		t.Fatal(err)
	}
	legacyStore, _ := settings.NewStore(filepath.Join(root, "settings.json"))
	old, err := legacyStore.Load()
	if err != nil || old.Mihomo.SelectedVersion != "" {
		t.Fatal("new settings written into legacy root", err)
	}
	if _, err := os.Stat(filepath.Join(root, "settings.json")); !os.IsNotExist(err) {
		t.Fatal("created legacy settings")
	}
	coreState, err := s.versionManager.State()
	if err != nil || coreState.DataDirectory != root || coreState.CoreDirectory != filepath.Join(root, "core", "mihomo") {
		t.Fatal("core moved with configuration", err)
	}
}

func workspaceConfigurationBytes(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, name := range []string{"settings.json", "subscriptions", "local-configs", "local-scripts", "chain-proxies", "geodata", "state/mihomo/proxy-selections"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		if err := filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			data, err := os.ReadFile(path)
			result[rel] = data
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	return result
}
