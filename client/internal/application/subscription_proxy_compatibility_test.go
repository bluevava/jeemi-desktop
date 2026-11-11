package application

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/subscription"
	"jeemi/internal/subscriptionformat"
)

const runtimeCompatibilityFixture = `# keep the complete downloaded subscription
proxies:
  - {name: Unsupported, type: snell, server: 192.0.2.1, port: 8390, version: 6, psk: fixture, userkey: fixture, mode: unsafe-raw}
  - {name: Supported, type: anytls, server: 192.0.2.2, port: 443, password: fixture}
proxy-groups:
  - {name: Proxy, type: select, proxies: [Unsupported, Supported]}
  - {name: Auto, type: url-test, proxies: [Unsupported], url: 'https://example.invalid/test'}
rules:
  - DOMAIN,example.invalid,Unsupported
  - MATCH,Proxy
`

func assertCompatibleProjection(t *testing.T, s *Service, state subscription.State, raw string) {
	t.Helper()
	if state.Projection == nil || state.Projection.Status != projectionReady || len(state.Projection.Proxies) != 1 || state.Projection.Proxies[0].Name != "Supported" {
		t.Fatal("compatible runtime projection was not generated")
	}
	report := state.Projection.Summary.Normalization
	if report.Format != "mihomo" || report.SkippedNodes != 0 || len(report.Diagnostics) != 0 {
		t.Fatal("runtime filtering added normalization diagnostics")
	}
	source, err := s.SubscriptionText(state.SelectedSubscriptionID)
	if err != nil || source.Contents != raw {
		t.Fatal("runtime filtering changed the subscription source", err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || strings.Contains(view.Contents, "Unsupported") || !strings.Contains(view.Contents, "DOMAIN,example.invalid,REJECT") {
		t.Fatal("resolved runtime still contains unsupported nodes or dangling targets", err)
	}
	result, err := subscriptionformat.Normalize([]byte(raw))
	if err != nil || string(result.Contents) != raw {
		t.Fatal("native standardization was changed", err)
	}
	snapshot, err := s.resolvedConfigs.Current(state.SelectedSubscriptionID)
	if err != nil || snapshot.Manifest.NormalizationFingerprint != result.Report.Fingerprint || state.Projection.NormalizationFingerprint != result.Report.Fingerprint {
		t.Fatal("runtime compatibility did not use the current derivation fingerprint", err)
	}
	if s.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("offline composition started the core")
	}
}

func TestNativeSubscriptionFiltersUnsupportedOnlyInRuntime(t *testing.T) {
	s, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	useDomainOnlyFakeIPFilter(t, s)
	path := filepath.Join(t.TempDir(), "mixed.yaml")
	if err := os.WriteFile(path, []byte(runtimeCompatibilityFixture), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCompatibleProjection(t, s, state, runtimeCompatibilityFixture)

	// Local scripts receive the original full configuration; both source and
	// script-added unsupported nodes are pruned only after that script completes.
	scriptSource := `function main(c) {
  if (c.proxies[0].type !== 'snell' || c.proxies[0].version !== 6) throw new Error('source was filtered early');
  c.proxies.push({name: 'ScriptUnsupported', type: 'unsupported-fixture'});
  c['proxy-groups'][0].proxies.push('ScriptUnsupported');
  c['x-script-ran'] = true;
  return c;
}`
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Original input", Contents: scriptSource})
	if err != nil {
		t.Fatal(err)
	}
	savedBefore, err := s.localScripts.Get(script.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.SetSubscriptionLocalScript(state.SelectedSubscriptionID, script.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertCompatibleProjection(t, s, state, runtimeCompatibilityFixture)
	config := normalizedRuntime(t, s)
	if config["x-script-ran"] != true || len(config["proxies"].([]any)) != 1 {
		t.Fatal("runtime filter did not run after the local script")
	}
	saved, err := s.localScripts.Get(script.ID)
	if err != nil || !reflect.DeepEqual(saved, savedBefore) {
		t.Fatal("runtime filtering rewrote the saved script", err)
	}
}

func TestNativeRefreshAcceptsUnsupportedNodesWithoutChangingSavedSource(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	fetcher.contents = runtimeCompatibilityFixture
	state, err := s.RefreshSubscription(id)
	if err != nil {
		t.Fatal(err)
	}
	assertCompatibleProjection(t, s, state, runtimeCompatibilityFixture)
	before, _ := s.Subscription(id)
	viewBefore, _ := s.RuntimeConfigurationText()

	// A separate configuration error must still reject the candidate and retain
	// both the previous raw revision and its valid resolved configuration.
	fetcher.contents = strings.Replace(runtimeCompatibilityFixture, "MATCH,Proxy", "MATCH,Missing", 1)
	if _, err := s.RefreshSubscription(id); err == nil {
		t.Fatal("unrelated missing target was silently ignored")
	}
	after, _ := s.Subscription(id)
	viewAfter, _ := s.RuntimeConfigurationText()
	if !reflect.DeepEqual(before, after) || viewAfter.Contents != viewBefore.Contents {
		t.Fatal("invalid refresh replaced the last valid revision")
	}

	// An entirely incompatible native source still refreshes, with no implicit
	// direct fallback and no native-format skip notification.
	fetcher.contents = "proxies: [{name: Unsupported, type: snell, version: 6}]\nproxy-groups: [{name: Proxy, type: select, proxies: [Unsupported]}]\nrules: ['MATCH,Proxy']\n"
	state, err = s.RefreshSubscription(id)
	if err != nil || state.Projection.Status != projectionReady || len(state.Projection.Proxies) != 0 {
		t.Fatal("fully filtered runtime was not safely composed", err)
	}
	config := normalizedRuntime(t, s)
	group := config["proxy-groups"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(group["proxies"], []any{"REJECT"}) || group["empty-fallback"] != "REJECT" {
		t.Fatal("empty selector became direct")
	}
	source, _ := s.SubscriptionText(id)
	if source.Contents != fetcher.contents || state.Projection.Summary.Normalization.SkippedNodes != 0 {
		t.Fatal("fully filtered runtime changed source or emitted a skip report")
	}
}
