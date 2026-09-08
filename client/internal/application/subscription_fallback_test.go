package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscription"
)

type fallbackFetcher struct {
	contents string
	err      error
}

func (f *fallbackFetcher) Fetch(context.Context, string) (subscription.FetchedContent, error) {
	return subscription.FetchedContent{Contents: []byte(f.contents), Format: subscription.FormatYAML}, f.err
}

const fallbackFixture = `proxy-groups:
  - {name: Original, type: select, proxies: [DIRECT]}
  - {name: Hidden, type: select, proxies: [DIRECT], hidden: true}
rules:
  - DOMAIN,example.org,DIRECT
  - MATCH,Original
`

func fallbackService(t *testing.T) (*Service, *fallbackFetcher, subscription.State) {
	t.Helper()
	dataDirectory := filepath.Join(t.TempDir(), "jeemi_data")
	s, err := NewService(dataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	f := &fallbackFetcher{contents: fallbackFixture}
	s.subscriptions, err = subscription.NewManager(subscription.ManagerOptions{DataDirectory: dataDirectory, Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/config"})
	if err != nil {
		t.Fatal(err)
	}
	return s, f, state
}

func TestSubscriptionFallbackIsPerSubscriptionAndNeverStartsCore(t *testing.T) {
	s, _, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	chosen := fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Hidden"}
	result, err := s.SetSubscriptionFallback(id, chosen, 0)
	state = result.State
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection.Fallback.OriginalTarget != "Original" || state.Projection.Fallback.Selection != chosen || state.Projection.FallbackOverrideRevision != 1 {
		t.Fatalf("wrong fallback projection: %+v", state.Projection)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(view.Contents, "MATCH,Hidden") || view.FallbackOverrideRevision != 1 {
		t.Fatalf("runtime injection not resolved: %+v %v", view, err)
	}
	source, _ := s.SubscriptionText(id)
	if source.Contents != fallbackFixture {
		t.Fatal("fallback changed immutable source")
	}
	if s.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("configuration edit started core")
	}
	if _, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeDirect}, 0); err == nil {
		t.Fatal("stale save overwrote newer selection")
	}
	if _, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Missing"}, 1); err == nil {
		t.Fatal("unavailable target was saved")
	}

	second, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Second", SourceURL: "https://example.test/second"})
	if err != nil {
		t.Fatal(err)
	}
	other := ""
	for _, item := range second.Subscriptions {
		if item.ID != id {
			other = item.ID
			if item.Fallback.Mode != fallbackoverride.ModeNone {
				t.Fatal("new subscription inherited override")
			}
		}
	}
	if _, err := s.SelectSubscription(other); err != nil {
		t.Fatal(err)
	}
	state, err = s.SelectSubscription(id)
	if err != nil || state.Projection.Fallback.Selection != chosen {
		t.Fatalf("switching lost per-subscription override: %+v %v", state.Projection, err)
	}
	preferences := runtimeconfig.DefaultPreferences()
	preferences.OutboundMode = runtimeconfig.OutboundModeGlobal
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Subscription(id)
	if detail.Fallback != chosen {
		t.Fatal("global mode cleared fallback preference")
	}
	preferences.OutboundMode = runtimeconfig.OutboundModeRule
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	result, err = s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeNone}, 1)
	state = result.State
	if err != nil || state.Projection.FallbackOverrideRevision != 2 {
		t.Fatalf("disable override failed: %v", err)
	}
	view, err = s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(view.Contents, "MATCH,Original") {
		t.Fatal("no override did not restore source terminal")
	}
}

func TestFallbackRefreshRetainsOnFailureAndPersistsMissingTargetReset(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	chosen := fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Hidden"}
	if _, err := s.SetSubscriptionFallback(id, chosen, 0); err != nil {
		t.Fatal(err)
	}
	fetcher.err = errors.New("fixture fetch failed")
	if _, err := s.RefreshSubscription(id); err == nil {
		t.Fatal("failed fetch accepted")
	}
	detail, _ := s.Subscription(id)
	if detail.Fallback != chosen || detail.FallbackOverrideRevision != 1 {
		t.Fatal("failed fetch cleared selection")
	}
	fetcher.err = nil
	fetcher.contents = "proxy-groups: {}\nrules: [MATCH,DIRECT]\n"
	before := detail.CurrentRevisionID
	if _, err := s.RefreshSubscription(id); err == nil {
		t.Fatal("invalid refreshed configuration was accepted")
	}
	detail, _ = s.Subscription(id)
	if detail.Fallback != chosen || detail.CurrentRevisionID != before {
		t.Fatal("failed composition changed the saved subscription")
	}
	fetcher.contents = strings.ReplaceAll(fallbackFixture, "Original", "New original")
	if _, err := s.RefreshSubscription(id); err != nil {
		t.Fatal(err)
	}
	detail, _ = s.Subscription(id)
	if detail.Fallback != chosen {
		t.Fatal("matching selector was reset")
	}
	fetcher.contents = "proxy-groups: [{name: New, type: select, proxies: [DIRECT]}]\nrules: [\"MATCH,New\"]\n"
	state, err := s.RefreshSubscription(id)
	if err != nil {
		t.Fatal(err)
	}
	detail, _ = s.Subscription(id)
	if detail.Fallback.Mode != fallbackoverride.ModeNone || detail.FallbackOverrideRevision != 2 || detail.FallbackResetTarget != "Hidden" || state.Projection.FallbackOverrideRevision != 2 {
		t.Fatalf("reset was not atomically reflected in returned state: %+v", detail)
	}
	if _, err := s.RefreshSubscription(id); err != nil {
		t.Fatal(err)
	}
	detail, _ = s.Subscription(id)
	if detail.FallbackOverrideRevision != 2 {
		t.Fatal("same reset was repeated")
	}
	if _, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeDirect}, 2); err != nil {
		t.Fatal(err)
	}
	fetcher.contents = "proxies: []\nrules: []\n"
	state, err = s.RefreshSubscription(id)
	if err != nil || state.Projection.Fallback.Selection.Mode != fallbackoverride.ModeDirect {
		t.Fatal("DIRECT was incorrectly treated as a missing selector")
	}
}

func TestScriptFallbackPreviewIsPureAndSavedScriptResetsMissingTarget(t *testing.T) {
	s, _, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	if _, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Hidden"}, 0); err != nil {
		t.Fatal(err)
	}
	before, err := s.resolvedConfigs.Current(id)
	if err != nil {
		t.Fatal(err)
	}
	contents := `function main(config) { config["proxy-groups"] = [{name: "Script group", type: "select", proxies: ["DIRECT"]}]; config.rules = ["MATCH,Script group"]; return config; }`
	if _, err := s.TestLocalScript(localscript.TestInput{SubscriptionID: id, SaveInput: localscript.SaveInput{Name: "Candidate", Contents: contents}}); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Subscription(id)
	after, _ := s.resolvedConfigs.Current(id)
	if detail.FallbackOverrideRevision != 1 || detail.Fallback.Selector != "Hidden" || before.Manifest.Fingerprint != after.Manifest.Fingerprint {
		t.Fatal("script preview wrote preferences or resolved snapshot")
	}
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Script", Contents: contents})
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.SetSubscriptionLocalScript(id, script.ID)
	if err != nil || state.Projection.Fallback.Selection.Mode != fallbackoverride.ModeNone {
		t.Fatalf("saved association failed to reset unavailable selector: %v", err)
	}
	result, err := s.SetSubscriptionFallback(id, fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Script group"}, 2)
	state = result.State
	if err != nil || state.Projection.Fallback.OriginalTarget != "Script group" {
		t.Fatalf("script selector is not available: %v", err)
	}
	if _, err := s.SaveLocalScript(localscript.SaveInput{ID: script.ID, Name: script.Name, Contents: "function main() { throw new Error('fixture'); }"}); err == nil {
		t.Fatal("invalid referenced script was saved")
	}
	detail, _ = s.Subscription(id)
	if detail.Fallback.Selector != "Script group" || detail.FallbackOverrideRevision != 3 {
		t.Fatal("failed script save reset fallback")
	}
	// Successful changes to an already referenced script use the same reset path.
	if _, err := s.SaveLocalScript(localscript.SaveInput{ID: script.ID, Name: script.Name, Contents: strings.ReplaceAll(contents, "Script group", "Renamed")}); err != nil {
		t.Fatal(err)
	}
	detail, _ = s.Subscription(id)
	if detail.Fallback.Mode != fallbackoverride.ModeNone || detail.FallbackOverrideRevision != 4 {
		t.Fatal("referenced script change did not persist reset")
	}
	if _, err := os.Stat(filepath.Join(s.subscriptions.Directory(), id, "metadata.json")); err != nil {
		t.Fatal(err)
	}
}
