package geodata

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type acceptingValidator struct {
	calls []Kind
	paths []string
}

func (validator *acceptingValidator) Validate(_ context.Context, _ string, kind Kind, path string) error {
	validator.calls = append(validator.calls, kind)
	validator.paths = append(validator.paths, path)
	return nil
}

func TestImportActivationAndRollbackKeepImmutableRevisions(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "jeemi_data")
	validator := &acceptingValidator{}
	manager, err := NewManager(ManagerOptions{DataDirectory: dataRoot, Validator: validator})
	if err != nil {
		t.Fatal(err)
	}
	preferences := DefaultPreferences()
	first := filepath.Join(t.TempDir(), "first.metadb")
	if err := os.WriteFile(first, []byte("first database"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := manager.Import(context.Background(), KindGeoIPMMDB, first, "mihomo", "v1", preferences)
	if err != nil {
		t.Fatal(err)
	}
	if !state.RestartRequired || len(validator.calls) != 1 || validator.calls[0] != KindGeoIPMMDB {
		t.Fatalf("import did not create a validated pending revision: %+v calls=%v", state, validator.calls)
	}
	if len(validator.paths) != 1 || filepath.Clean(validator.paths[0]) == filepath.Clean(first) {
		t.Fatalf("import validation did not use a private staged copy: %v", validator.paths)
	}
	activation, err := manager.PrepareActivation(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if err := activation.Commit(); err != nil {
		t.Fatal(err)
	}
	activePath := filepath.Join(dataRoot, "runtime", "mihomo", "geoip.metadb")
	if contents, err := os.ReadFile(activePath); err != nil || string(contents) != "first database" {
		t.Fatalf("first revision was not activated: contents=%q err=%v", contents, err)
	}

	second := filepath.Join(t.TempDir(), "second.metadb")
	if err := os.WriteFile(second, []byte("second database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Import(context.Background(), KindGeoIPMMDB, second, "mihomo", "v2", preferences); err != nil {
		t.Fatal(err)
	}
	activation, err = manager.PrepareActivation(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(activePath); err != nil || string(contents) != "second database" {
		t.Fatalf("second revision was not staged: contents=%q err=%v", contents, err)
	}
	if err := activation.Rollback(); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(activePath); err != nil || string(contents) != "first database" {
		t.Fatalf("rollback did not restore first revision: contents=%q err=%v", contents, err)
	}
	state, err = manager.State(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if !state.RestartRequired {
		t.Fatal("rollback lost the newer desired revision")
	}
}

func TestActiveRequirementDetectsTampering(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "jeemi_data")
	manager, err := NewManager(ManagerOptions{DataDirectory: dataRoot, Validator: &acceptingValidator{}})
	if err != nil {
		t.Fatal(err)
	}
	preferences := DefaultPreferences()
	source := filepath.Join(t.TempDir(), "geoip.metadb")
	if err := os.WriteFile(source, []byte("valid database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Import(context.Background(), KindGeoIPMMDB, source, "mihomo", "v1", preferences); err != nil {
		t.Fatal(err)
	}
	activation, err := manager.PrepareActivation(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if err := activation.Commit(); err != nil {
		t.Fatal(err)
	}
	configuration := []byte("rules:\n  - GEOIP,CN,DIRECT\n  - MATCH,DIRECT\n")
	if err := manager.ValidateActiveRequirements(configuration, preferences); err != nil {
		t.Fatalf("valid active asset was rejected: %v", err)
	}
	activePath := filepath.Join(dataRoot, "runtime", "mihomo", "geoip.metadb")
	if err := os.WriteFile(activePath, []byte("tampered database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateActiveRequirements(configuration, preferences); err == nil {
		t.Fatal("tampered active GEO data was accepted")
	}
}

func TestHealthDistinguishesMissingPendingReadyAndInvalidAssets(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "jeemi_data")
	manager, err := NewManager(ManagerOptions{DataDirectory: dataRoot, Validator: &acceptingValidator{}})
	if err != nil {
		t.Fatal(err)
	}
	preferences := DefaultPreferences()
	health, err := manager.Health(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != HealthMissing || len(health.MissingKinds) != 3 {
		t.Fatalf("fresh GEO health = %+v", health)
	}

	for _, kind := range SelectedKinds(preferences) {
		source := filepath.Join(t.TempDir(), string(kind))
		if err := os.WriteFile(source, []byte("database-"+string(kind)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Import(context.Background(), kind, source, "mihomo", "v1", preferences); err != nil {
			t.Fatal(err)
		}
	}
	health, err = manager.Health(preferences)
	if err != nil || health.Status != HealthPending {
		t.Fatalf("pending GEO health = %+v, err = %v", health, err)
	}
	activation, err := manager.PrepareActivation(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if err := activation.Commit(); err != nil {
		t.Fatal(err)
	}
	health, err = manager.Health(preferences)
	if err != nil || health.Status != HealthReady {
		t.Fatalf("ready GEO health = %+v, err = %v", health, err)
	}
	if err := os.Remove(filepath.Join(dataRoot, "runtime", "mihomo", "GeoSite.dat")); err != nil {
		t.Fatal(err)
	}
	health, err = manager.Health(preferences)
	if err != nil || health.Status != HealthInvalid || len(health.InvalidKinds) != 1 || health.InvalidKinds[0] != KindGeoSite {
		t.Fatalf("invalid GEO health = %+v, err = %v", health, err)
	}
}

func TestPreferencesRequireSafeCustomURLs(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.Source = SourceCustom
	preferences.CustomURLs.GeoSite = "http://example.test/geosite.dat"
	if err := ValidatePreferences(preferences); err == nil {
		t.Fatal("insecure custom URL was accepted")
	}
	preferences.CustomURLs.GeoSite = "https://user@example.test/geosite.dat"
	if err := ValidatePreferences(preferences); err == nil {
		t.Fatal("custom URL containing user information was accepted")
	}
	preferences.CustomURLs.GeoSite = ""
	if normalized := Normalize(preferences); normalized.CustomURLs.GeoSite != "" {
		t.Fatal("normalization silently replaced an explicitly empty custom URL")
	}
}

func TestDownloadRedirectKeepsSafeHTTPSBoundary(t *testing.T) {
	client, ok := defaultHTTPClient().(*http.Client)
	if !ok || client.CheckRedirect == nil {
		t.Fatal("default GEO data client has no redirect policy")
	}
	for _, rawURL := range []string{
		"http://example.test/geosite.dat",
		"https://user@example.test/geosite.dat",
	} {
		request, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.CheckRedirect(request, nil); err == nil {
			t.Fatalf("unsafe redirect was accepted: %s", rawURL)
		}
	}
	request, err := http.NewRequest(http.MethodGet, "https://example.test/geosite.dat", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(request, nil); err != nil {
		t.Fatalf("safe redirect was rejected: %v", err)
	}
}

func TestSidecarURLAppendsBeforeQuery(t *testing.T) {
	actual, err := sidecarURL("https://example.test/geosite.dat?channel=stable#ignored")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "https://example.test/geosite.dat.sha256sum?channel=stable" {
		t.Fatalf("unexpected checksum URL: %s", actual)
	}
}
