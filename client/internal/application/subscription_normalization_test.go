package application

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/compose"
	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscriptionformat"
)

const normalizedURIFixture = "host://*.entry.test?server=https%3A%2F%2Fdns.example.test%2Fdns-query\n" +
	"anytls://fixture-pass@a.entry.test:443?sni=tls.example.test&fp=chrome#Node\n"

func normalizedRuntime(t *testing.T, service *Service) map[string]any {
	t.Helper()
	view, err := service.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal([]byte(view.Contents), &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestConvertedSubscriptionSharesLocalAndRuntimeComposition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "jeemi_data")
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	raw := "\ufeff" + base64.StdEncoding.EncodeToString([]byte(normalizedURIFixture)) + "\r\n"
	path := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	id := state.SelectedSubscriptionID
	projection := state.Projection
	if projection.Status != projectionReady || projection.Summary.Normalization.Format != "uri" || len(projection.Selectors) != 1 || projection.Selectors[0].Name != "Proxy" {
		t.Fatal("converted import did not reach the shared projection")
	}
	text, err := s.SubscriptionText(id)
	if err != nil || text.Contents != raw {
		t.Fatal("import changed raw bytes", err)
	}
	result, _ := subscriptionformat.Normalize([]byte(raw))
	snapshot, err := s.resolvedConfigs.Current(id)
	if err != nil || snapshot.Manifest.NormalizationFingerprint != result.Report.Fingerprint || projection.NormalizationFingerprint != result.Report.Fingerprint {
		t.Fatal("normalization identity was not retained", err)
	}
	config := normalizedRuntime(t, s)
	dns := config["dns"].(map[string]any)
	for _, field := range []string{"nameserver-policy", "proxy-server-nameserver-policy"} {
		if !reflect.DeepEqual(dns[field].(map[string]any)["*.entry.test"], []any{"https://dns.example.test/dns-query"}) {
			t.Fatal("entry DNS policy was lost", field)
		}
	}
	if !reflect.DeepEqual(dns["proxy-server-nameserver"], []any{"system"}) || len(dns["nameserver"].([]any)) == 0 {
		t.Fatal("node DNS defaults or home nameservers were lost")
	}

	local, err := s.SaveLocalConfig(profile.SaveLocalConfigInput{Name: "Local", Fields: []profile.LocalConfigField{{Path: "/sniffer/enable", ValueYAML: "true", Strategy: compose.StrategyReplace}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(id, local.ID); err != nil {
		t.Fatal(err)
	}
	config = normalizedRuntime(t, s)
	if config["sniffer"].(map[string]any)["enable"] != true || len(config["proxies"].([]any)) != 1 {
		t.Fatal("local configuration did not compose over converted nodes")
	}
	if _, err := s.SetSubscriptionLocalConfig(id, ""); err != nil {
		t.Fatal(err)
	}
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Check normalized input", Contents: `function main(config) {
  if (config.proxies[0].type !== 'anytls' || config.rules[0] !== 'MATCH,Proxy' ||
      config.dns['proxy-server-nameserver-policy']['*.entry.test'][0] !== 'https://dns.example.test/dns-query') throw new Error('missing conversion');
  config.dns.nameserver = ['https://script.example.test/dns-query'];
  config.mode = 'direct'; return config;
}`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
		t.Fatal(err)
	}
	config = normalizedRuntime(t, s)
	dns = config["dns"].(map[string]any)
	if config["mode"] != "rule" || dns["nameserver"].([]any)[0] != "https://script.example.test/dns-query" {
		t.Fatal("script/runtime precedence is incorrect")
	}

	prefs := runtimeconfig.DefaultPreferences()
	prefs.DNSProxyServerNameserverPolicyEnabled = true
	prefs.DNSProxyServerNameserverPolicyMerge = runtimeconfig.MergeModeOverride
	prefs.DNSProxyServerNameserverPolicyYAML = "'*.entry.test': [https://home.example.test/dns-query]"
	if _, err := s.SaveRuntimePreferences(prefs); err != nil {
		t.Fatal(err)
	}
	dns = normalizedRuntime(t, s)["dns"].(map[string]any)
	if !reflect.DeepEqual(dns["proxy-server-nameserver-policy"].(map[string]any)["*.entry.test"], []any{"https://home.example.test/dns-query"}) {
		t.Fatal("normalizer reinserted DNS after explicit home override")
	}
	text, _ = s.SubscriptionText(id)
	if text.Contents != raw || s.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("composition changed source or started the core")
	}
}

func TestConvertedImportCoreRequirementFailsBeforePersistence(t *testing.T) {
	s, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.settingsStore.SetMihomoVersion("v1.19.20"); err != nil {
		t.Fatal(err)
	}
	raw := "vless://01234567-89ab-cdef-0123-456789abcdef@node.example.test:443?security=tls&type=xhttp&path=%2F&mode=auto#Node"
	path := filepath.Join(t.TempDir(), "advanced.txt")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportSubscriptionFile(path); err == nil || !strings.Contains(err.Error(), "core_version_required") {
		t.Fatal("old core accepted advanced fields", err)
	}
	state, err := s.subscriptions.State()
	if err != nil || len(state.Subscriptions) != 0 {
		t.Fatal("rejected import published source", err)
	}
	if _, err := s.settingsStore.SetMihomoVersion("v1.19.30"); err != nil {
		t.Fatal(err)
	}
	state, err = s.ImportSubscriptionFile(path)
	if err != nil || state.Projection.Status != projectionReady {
		t.Fatal("supported version did not produce offline projection", err)
	}
	if _, err := s.settingsStore.SetMihomoVersion("unknown-fixture"); err != nil {
		t.Fatal(err)
	}
	state, err = s.SubscriptionState()
	if err != nil || state.Projection.Summary.NormalizationError.Code != "core_version_required" {
		t.Fatal("version change reused incompatible projection", err)
	}
}

func TestConvertedRefreshCandidateUsesNewBytesAndPreservesOnFailure(t *testing.T) {
	s, fetcher, state := fallbackService(t)
	id := state.SelectedSubscriptionID
	fetcher.contents = normalizedURIFixture
	state, err := s.RefreshSubscription(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.Projection.Summary.Normalization.Format != "uri" || len(state.Projection.Proxies) != 1 {
		t.Fatal("refresh used old native source")
	}
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Require node", Contents: "function main(c) { if (c.proxies[0].name !== 'Node') throw new Error('changed'); return c; }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Subscription(id)
	resolved, _ := s.RuntimeConfigurationText()
	fetcher.contents = strings.ReplaceAll(normalizedURIFixture, "#Node", "#Changed")
	result, err := s.CheckSubscriptionRefresh(id)
	if err != nil || result.Conflict == nil {
		t.Fatal("converted candidate did not reach script", err)
	}
	if _, err := s.ResolveSubscriptionRefresh(result.Conflict.Token, false); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Subscription(id)
	view, _ := s.RuntimeConfigurationText()
	if !reflect.DeepEqual(before, after) || view.Contents != resolved.Contents {
		t.Fatal("rejected refresh changed saved or resolved source")
	}
	fetcher.contents = "host://*.entry.test?server=broken\n" + normalizedURIFixture
	result, err = s.CheckSubscriptionRefresh(id)
	if err == nil || result.Conflict != nil {
		t.Fatal("invalid DNS offered local detachment")
	}
	after, _ = s.Subscription(id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid source changed success metadata")
	}
	fetcher.contents = "[Proxy]\nSurge = socks5, 192.0.2.1, 1080, a, b\n"
	result, err = s.CheckSubscriptionRefresh(id)
	if err != nil || result.Conflict == nil {
		t.Fatal("Surge candidate not classified", err)
	}
	state, err = s.ResolveSubscriptionRefresh(result.Conflict.Token, true)
	if err != nil || state.Projection.Summary.Normalization.Format != "surge" || state.Projection.LocalScriptID != "" {
		t.Fatal("checked Surge source was not committed", err)
	}
	text, _ := s.SubscriptionText(id)
	if text.Contents != fetcher.contents {
		t.Fatal("refresh saved converted YAML instead of raw Surge")
	}
}

func TestNormalizationChangeCannotUseRuntimeModeFastPath(t *testing.T) {
	a := mihomoruntime.Source{SubscriptionID: strings.Repeat("a", 32), NormalizationFingerprint: strings.Repeat("b", 64)}
	b := a
	b.NormalizationFingerprint = strings.Repeat("c", 64)
	if sameRuntimeSource(a, b) {
		t.Fatal("changed conversion reused active source")
	}
}
