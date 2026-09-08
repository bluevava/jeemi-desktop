package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/geodata"
	"jeemi/internal/runtimeconfig"
)

func TestStorePersistsMihomoSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if _, err := store.SetMihomoVersion("v1.19.30"); err != nil {
		t.Fatalf("SetMihomoVersion() error = %v", err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if document.Mihomo.SelectedVersion != "v1.19.30" {
		t.Fatalf("SelectedVersion = %q", document.Mihomo.SelectedVersion)
	}
}

func TestNewStoreRejectsRelativePath(t *testing.T) {
	if _, err := NewStore("settings.json"); err == nil {
		t.Fatal("NewStore() accepted a relative path")
	}
}

func TestStorePersistsSubscriptionSelectionAndDensityWithoutOverwritingMihomo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMihomoVersion("v1.19.30"); err != nil {
		t.Fatal(err)
	}
	selectedID := "0123456789abcdef0123456789abcdef"
	if _, err := store.SetSelectedSubscription(selectedID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSubscriptionSelectorDisplayPreferences(SelectorSortDelay, SelectorViewTabs); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSubscriptionPreferences(SelectorDensitySmall, 12, ConnectionResetAll); err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if document.Mihomo.SelectedVersion != "v1.19.30" || document.Subscriptions.SelectedID != selectedID || document.Subscriptions.SelectorDensity != SelectorDensitySmall || document.Subscriptions.DelayTestConcurrency != 12 || document.Subscriptions.ConnectionResetMode != ConnectionResetAll || document.Subscriptions.SelectorSortMode != SelectorSortDelay || document.Subscriptions.SelectorViewMode != SelectorViewTabs {
		t.Fatalf("settings fields overwrote each other: %+v", document)
	}
	if _, err := store.SetSubscriptionPreferences("dense", 12, ConnectionResetSelector); err == nil {
		t.Fatal("SetSubscriptionPreferences() accepted an unsupported density")
	}
	if _, err := store.SetSubscriptionPreferences(SelectorDensityMedium, 0, ConnectionResetSelector); err == nil {
		t.Fatal("SetSubscriptionPreferences() accepted invalid concurrency")
	}
	if _, err := store.SetSubscriptionPreferences(SelectorDensityMedium, 12, "invalid"); err == nil {
		t.Fatal("SetSubscriptionPreferences() accepted an unsupported connection reset mode")
	}
	if _, err := store.SetSubscriptionSelectorDisplayPreferences("random", SelectorViewPanel); err == nil {
		t.Fatal("SetSubscriptionSelectorDisplayPreferences() accepted an unsupported sort mode")
	}
	if _, err := store.SetSubscriptionSelectorDisplayPreferences(SelectorSortDefault, "cards"); err == nil {
		t.Fatal("SetSubscriptionSelectorDisplayPreferences() accepted an unsupported view mode")
	}
}

func TestStoreDefaultsMissingSubscriptionPreferencesWithoutRewriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte(`{"subscriptions":{"selectorDensity":"medium"}}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if document.Subscriptions.SelectorDensity != SelectorDensityMedium || document.Subscriptions.DelayTestConcurrency != DefaultDelayTestConcurrency || document.Subscriptions.ConnectionResetMode != DefaultConnectionResetMode || document.Subscriptions.SelectorSortMode != DefaultSelectorSortMode || document.Subscriptions.SelectorViewMode != DefaultSelectorViewMode {
		t.Fatalf("subscription preferences were not normalised: %+v", document.Subscriptions)
	}
	if again, err := store.Load(); err != nil || !reflect.DeepEqual(document, again) {
		t.Fatal("repeated settings read changed the defaults", err)
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != string(contents) {
		t.Fatal("reading settings rewrote the file", err)
	}
}

func TestStoreUsesMediumAsTheDefaultSelectorDensity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if document.Subscriptions.SelectorDensity != SelectorDensityMedium {
		t.Fatalf("default selector density = %q", document.Subscriptions.SelectorDensity)
	}
}

func TestStorePersistsRuntimePreferencesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMihomoVersion("v1.19.30"); err != nil {
		t.Fatal(err)
	}
	selectedID := "0123456789abcdef0123456789abcdef"
	if _, err := store.SetSelectedSubscription(selectedID); err != nil {
		t.Fatal(err)
	}
	preferences := runtimeconfig.DefaultPreferences()
	preferences.OutboundMode = runtimeconfig.OutboundModeGlobal
	preferences.ProxyMode = runtimeconfig.ProxyModeTUN
	preferences.ListenerType = runtimeconfig.ListenerTypeSOCKS
	preferences.ListenPort = 1080
	preferences.AllowLAN = true
	preferences.TUNStack = runtimeconfig.TUNStackGVisor
	preferences.TUNRouteExcludeAddress = []string{"192.168.50.0/24", "fd00:50::/64"}
	preferences.TUNRouteExcludeAddressEnabled = true
	preferences.TUNRouteExcludeAddressMerge = runtimeconfig.MergeModeOverride
	preferences.LogLevel = runtimeconfig.LogLevelDebug
	preferences.DNSListen = "127.0.0.1:5353"
	preferences.DNSIPv6 = true
	preferences.DNSUseHosts = true
	preferences.DNSNameservers = []string{"https://dns.alidns.com/dns-query"}
	preferences.DNSNameserverMerge = runtimeconfig.MergeModeOverride
	preferences.DNSProxyServerNameserverEnabled = true
	preferences.DNSProxyServerNameservers = []string{}
	preferences.DNSNameserverPolicyEnabled = true
	preferences.DNSNameserverPolicyYAML = `"+.example.test": 223.5.5.5`
	preferences.HostsYAML = `"service.test": 127.0.0.1`
	preferences.LANBypassRules = []string{}
	if _, err := store.SetRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document.Runtime, preferences) {
		t.Fatalf("runtime preferences were not persisted: %+v", document.Runtime)
	}
	if document.Mihomo.SelectedVersion != "v1.19.30" || document.Subscriptions.SelectedID != selectedID {
		t.Fatalf("runtime preferences overwrote unrelated settings: %+v", document)
	}
	invalid := preferences
	invalid.ListenPort = 70000
	if _, err := store.SetRuntimePreferences(invalid); err == nil {
		t.Fatal("SetRuntimePreferences() accepted an invalid port")
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Runtime, preferences) {
		t.Fatalf("invalid update replaced valid preferences: %+v", reloaded.Runtime)
	}
}

func TestStoreDefaultsMissingRuntimePreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mihomo":{},"subscriptions":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document.Runtime, runtimeconfig.DefaultPreferences()) {
		t.Fatalf("missing runtime preferences were not defaulted: %+v", document.Runtime)
	}
	if !reflect.DeepEqual(document.GeoData, geodata.DefaultPreferences()) {
		t.Fatalf("missing GEO data preferences were not defaulted: %+v", document.GeoData)
	}
}

func TestStorePersistsGeoDataPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jeemi_data", "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	preferences := geodata.DefaultPreferences()
	preferences.GeoIPMode = geodata.GeoIPModeDAT
	preferences.Loader = geodata.LoaderStandard
	preferences.Source = geodata.SourceCustom
	preferences.CustomURLs.GeoIPDAT = "https://example.test/GeoIP.dat"
	if _, err := store.SetGeoDataPreferences(preferences); err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document.GeoData, preferences) {
		t.Fatalf("GEO data preferences were not persisted: %+v", document.GeoData)
	}
	invalid := preferences
	invalid.CustomURLs.GeoSite = "http://example.test/GeoSite.dat"
	if _, err := store.SetGeoDataPreferences(invalid); err == nil {
		t.Fatal("SetGeoDataPreferences() accepted an insecure custom URL")
	}
}
