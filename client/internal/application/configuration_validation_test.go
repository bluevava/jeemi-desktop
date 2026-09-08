package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"jeemi/internal/config/compose"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscription"
)

func TestLocalConfigCandidateFailurePreservesRevisionAndResolved(t *testing.T) {
	s, _, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(id, local.ID); err != nil {
		t.Fatal(err)
	}
	before, err := s.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	// A structurally valid sub-rule is invalid only after resolving its target.
	_, err = s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: "Broken edit", Fields: []profile.LocalConfigField{{
		Path: "/sub-rules", ValueYAML: "example: ['MATCH,missing-selector']", Strategy: compose.StrategyReplace,
	}}})
	if err == nil {
		t.Fatal("invalid final configuration was persisted")
	}
	after, err := s.LocalConfig(local.ID)
	if err != nil || !reflect.DeepEqual(local, after) {
		t.Fatal("failed candidate changed the saved local configuration", err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || view.Contents != before.Contents {
		t.Fatal("failed candidate changed resolved snapshot", err)
	}
	if _, err := s.SubscriptionState(); err != nil {
		t.Fatal("failed candidate blocked subscription page", err)
	}
}

func TestSharedResourceChecksInactiveSubscriptionsAndPreservesLibrary(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	first := state.SelectedSubscriptionID
	library, err := s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Local", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	group := library.StrategyGroups[0]
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{group.ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Reusable", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(first, local.ID); err != nil {
		t.Fatal(err)
	}
	fetcher.contents = "proxy-groups: [{name: Collision, type: select, proxies: [DIRECT]}]\nrules: ['MATCH,DIRECT']\n"
	secondState, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Inactive", SourceURL: "https://example.test/second"})
	if err != nil {
		t.Fatal(err)
	}
	second := ""
	for _, item := range secondState.Subscriptions {
		if item.ID != first {
			second = item.ID
		}
	}
	if _, err := s.SetSubscriptionLocalConfig(second, local.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectSubscription(first); err != nil {
		t.Fatal(err)
	}
	group.Name = "Collision"
	if _, err := s.SaveStrategyGroup(group); err == nil {
		t.Fatal("inactive subscription was not checked")
	}
	after, err := s.LocalConfigResources()
	if err != nil || !reflect.DeepEqual(library, after) {
		t.Fatal("rejected resource changed the library", err)
	}
	if s.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("candidate check started the core")
	}
}

func TestInvalidSavedAssociationRemainsEditableAndCanBeDetached(t *testing.T) {
	s, _, state := fallbackService(t)
	library, err := s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Local", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	group := library.StrategyGroups[0]
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{group.ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Repairable", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	// Simulate an old missing reference through the domain store. Normal UI
	// deletion is protected by the application service.
	if _, err := s.configResources.DeleteStrategyGroup(group.ID); err != nil {
		t.Fatal(err)
	}
	configs, err := s.LocalConfigState()
	if err != nil || len(configs.Configs) != 1 {
		t.Fatal("bad reference hid configuration list", err)
	}
	editable, err := s.LocalConfig(local.ID)
	if err != nil || editable.ResourcePlan.StrategyGroupIDs[0] != group.ID {
		t.Fatal("bad reference cannot be opened for repair", err)
	}
	broken, err := s.SubscriptionState()
	if err != nil || broken.Projection.Status == projectionReady {
		t.Fatal("bad projection hid subscription controls", err)
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, ""); err != nil {
		t.Fatal("cannot detach broken local configuration", err)
	}
	if _, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: local.Name}); err != nil {
		t.Fatal("cannot repair invalid configuration", err)
	}
}

type rejectingCandidateDriver struct {
	authorizationDriver
	validations int
}

func (d *rejectingCandidateDriver) Validate(context.Context, string, string, string) error {
	d.validations++
	return errors.New("fixture core rejected candidate")
}

func TestSelectedCoreRejectionPreventsLocalConfigSaveAndAssociation(t *testing.T) {
	s, _ := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	state, _ := s.SubscriptionState()
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	driver := &rejectingCandidateDriver{}
	root := filepath.Dir(s.localConfigs.Directory())
	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{DataDirectory: root, Driver: driver, SystemProxy: fallbackTestSystemProxy{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{ID: local.ID, Name: "Changed"}); err == nil {
		t.Fatal("core rejection was ignored")
	}
	after, _ := s.LocalConfig(local.ID)
	if after.Revision != local.Revision || after.Name != local.Name {
		t.Fatal("rejected candidate was persisted")
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, local.ID); err == nil {
		t.Fatal("core rejection saved association")
	}
	if driver.validations < 2 || driver.starts.Load() != 0 {
		t.Fatal("candidate validation did not use only the fake validator")
	}
}

func TestRuleSetCandidateConflictPreservesPayloadAndLibraryRevision(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	fetcher.contents = "rule-providers: {Remote: {type: inline, behavior: domain, payload: [example.org]}}\nrules: ['MATCH,DIRECT']\n"
	if _, err := s.RefreshSubscription(state.SelectedSubscriptionID); err != nil {
		t.Fatal(err)
	}
	library, err := s.SaveRuleSet(configresources.RuleSet{Name: "Local rules", SourceType: "inline", Behavior: "classical", PayloadYAML: "payload:\n  # keep this comment\n  - DOMAIN,example.com\n"})
	if err != nil {
		t.Fatal(err)
	}
	rules := library.RuleSets[0]
	library, err = s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Routing", Kind: "rule", Policy: configresources.PolicyTarget{Mode: "direct"}, RuleSetReferences: []configresources.RuleSetReference{{RuleSetID: rules.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{library.StrategyGroups[0].ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, local.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(library.Directory, "library.json"))
	rules.Name = "Remote"
	if _, err := s.SaveRuleSet(rules); err == nil {
		t.Fatal("conflicting rule provider name was saved")
	}
	after, _ := os.ReadFile(filepath.Join(library.Directory, "library.json"))
	if string(before) != string(after) {
		t.Fatal("rejected rule-set edit changed stored bytes")
	}
}

func TestRefreshConflictResolutionKeepsOrCommitsExactCandidate(t *testing.T) {
	for _, choice := range []string{"reject", "detach", "stale", "expired", "source-invalid"} {
		t.Run(choice, func(t *testing.T) {
			s, fetcher, state := fallbackService(t)
			id := state.SelectedSubscriptionID
			script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Conditional", Contents: "function main(config) { if (config.newSource) config['proxy-groups'] = {}; return config; }"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
				t.Fatal(err)
			}
			before, _ := s.Subscription(id)
			fetched := fallbackFixture + "newSource: true\n"
			if choice == "source-invalid" {
				fetched = "proxy-groups: {}\n"
			}
			fetcher.contents = fetched
			result, err := s.CheckSubscriptionRefresh(id)
			afterCheck, _ := s.Subscription(id)
			if !reflect.DeepEqual(before, afterCheck) {
				t.Fatal("checking refresh changed metadata or revision")
			}
			if choice == "source-invalid" {
				if err == nil || result.Conflict != nil {
					t.Fatal("invalid source offered detachment")
				}
				return
			}
			if err != nil || result.Conflict == nil {
				t.Fatal("handler conflict was not classified", err)
			}
			// A confirmation must never fetch different content.
			fetcher.contents = "proxies: []\n"
			if choice == "stale" {
				if _, err := s.subscriptions.Update(context.Background(), subscription.UpdateInput{ID: id, Name: "Renamed", SourceURL: before.SourceURL}); err != nil {
					t.Fatal(err)
				}
			}
			if choice == "expired" {
				s.pendingRefresh.expires = time.Now().Add(-time.Second)
			}
			_, err = s.ResolveSubscriptionRefresh(result.Conflict.Token, choice != "reject")
			if choice == "stale" || choice == "expired" {
				if err == nil {
					t.Fatal("stale confirmation overwrote a later change")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			after, _ := s.Subscription(id)
			text, _ := s.SubscriptionText(id)
			if choice == "detach" {
				if after.LocalScriptID != "" || text.Contents != fetched {
					t.Fatal("did not atomically detach and commit the checked bytes")
				}
			} else if after.CurrentRevisionID != before.CurrentRevisionID || after.LocalScriptID != script.ID {
				t.Fatal("rejected refresh changed source or association")
			}
		})
	}
}

func TestRefreshedSourceConflictsWithLocalSelectorAndCanDetach(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	library, err := s.SaveStrategyGroup(configresources.StrategyGroup{Name: "Local", Kind: "selector", Type: "select"})
	if err != nil {
		t.Fatal(err)
	}
	plan := configresources.DefaultPlan()
	plan.StrategyGroupIDs = []string{library.StrategyGroups[0].ID}
	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local config", ResourcePlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(id, local.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Subscription(id)
	fetcher.contents = "proxy-groups: [{name: Local, type: select, proxies: [DIRECT]}]\nrules: ['MATCH,Local']\n"
	// Changing a URL must validate the new source before writing metadata too.
	if _, err := s.UpdateSubscription(subscription.UpdateInput{ID: id, Name: "New name", SourceURL: "https://example.test/changed"}); err == nil {
		t.Fatal("URL edit saved a conflicting source")
	}
	unchanged, _ := s.Subscription(id)
	if !reflect.DeepEqual(before, unchanged) {
		t.Fatal("rejected URL edit changed the old subscription")
	}
	result, err := s.CheckSubscriptionRefresh(id)
	if err != nil || result.Conflict == nil || result.Conflict.LocalConfigID != local.ID {
		t.Fatal("local configuration conflict was not classified", err)
	}
	if _, err := s.ResolveSubscriptionRefresh(result.Conflict.Token, true); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Subscription(id)
	text, _ := s.SubscriptionText(id)
	if after.LocalConfigID != "" || text.Contents != fetcher.contents {
		t.Fatal("local configuration was not detached with the exact source revision")
	}
}
