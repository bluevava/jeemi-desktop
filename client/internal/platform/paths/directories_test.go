package paths

import (
	"path/filepath"
	"testing"
)

func TestDataDirectoryFromConfigRoot(t *testing.T) {
	configRoot := t.TempDir()
	got, err := dataDirectoryFromConfigRoot(configRoot)
	if err != nil {
		t.Fatalf("dataDirectoryFromConfigRoot() error = %v", err)
	}
	want := filepath.Join(configRoot, DataDirectoryName)
	if got != want {
		t.Fatalf("dataDirectoryFromConfigRoot() = %q, want %q", got, want)
	}
}

func TestDataDirectoryUsesJeemiDataName(t *testing.T) {
	got, err := DataDirectory()
	if err != nil {
		t.Fatalf("DataDirectory() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("DataDirectory() = %q, want absolute path", got)
	}
	if filepath.Base(got) != "jeemi_data" {
		t.Fatalf("DataDirectory() base = %q, want jeemi_data", filepath.Base(got))
	}
}

func TestProgramDirectoryFromExecutable(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "bin", "Jeemi.exe")
	want := filepath.Dir(executable)
	got, err := programDirectoryFromExecutable(executable)
	if err != nil {
		t.Fatalf("programDirectoryFromExecutable() error = %v", err)
	}
	if got != want {
		t.Fatalf("programDirectoryFromExecutable() = %q, want %q", got, want)
	}
}

func TestManagedDataPathsStayBelowJeemiData(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), DataDirectoryName)

	webView, err := WebViewDataDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("WebViewDataDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "webview2"); webView != want {
		t.Fatalf("WebViewDataDirectoryFromRoot() = %q, want %q", webView, want)
	}

	mihomo, err := MihomoDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("MihomoDirectoryFromRoot() error = %v", err)
	}
	geoData, err := GeoDataDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("GeoDataDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "geodata"); geoData != want {
		t.Fatalf("GeoDataDirectoryFromRoot() = %q, want %q", geoData, want)
	}

	localConfigs, err := LocalConfigsDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("LocalConfigsDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "local-configs"); localConfigs != want {
		t.Fatalf("LocalConfigsDirectoryFromRoot() = %q, want %q", localConfigs, want)
	}
	localScripts, err := LocalScriptsDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("LocalScriptsDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "local-scripts"); localScripts != want {
		t.Fatalf("LocalScriptsDirectoryFromRoot() = %q, want %q", localScripts, want)
	}
	resources, err := LocalConfigResourcesFileFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("LocalConfigResourcesFileFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "local-configs", "resources", "library.json"); resources != want {
		t.Fatalf("LocalConfigResourcesFileFromRoot() = %q, want %q", resources, want)
	}

	subscriptions, err := SubscriptionsDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("SubscriptionsDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "subscriptions"); subscriptions != want {
		t.Fatalf("SubscriptionsDirectoryFromRoot() = %q, want %q", subscriptions, want)
	}
	if want := filepath.Join(dataRoot, "core", "mihomo"); mihomo != want {
		t.Fatalf("MihomoDirectoryFromRoot() = %q, want %q", mihomo, want)
	}

	resolved, err := MihomoResolvedDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("MihomoResolvedDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "runtime", "mihomo", "resolved"); resolved != want {
		t.Fatalf("MihomoResolvedDirectoryFromRoot() = %q, want %q", resolved, want)
	}

	delayCache, err := MihomoProxyDelayCacheDirectoryFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("MihomoProxyDelayCacheDirectoryFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "cache", "mihomo", "proxy-delays"); delayCache != want {
		t.Fatalf("MihomoProxyDelayCacheDirectoryFromRoot() = %q, want %q", delayCache, want)
	}

	settings, err := SettingsFileFromRoot(dataRoot)
	if err != nil {
		t.Fatalf("SettingsFileFromRoot() error = %v", err)
	}
	if want := filepath.Join(dataRoot, "settings.json"); settings != want {
		t.Fatalf("SettingsFileFromRoot() = %q, want %q", settings, want)
	}
}

func TestManagedDataPathsRejectRelativeRoot(t *testing.T) {
	if _, err := MihomoDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("MihomoDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := GeoDataDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("GeoDataDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := LocalConfigsDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("LocalConfigsDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := LocalScriptsDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("LocalScriptsDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := LocalConfigResourcesFileFromRoot("jeemi_data"); err == nil {
		t.Fatal("LocalConfigResourcesFileFromRoot() accepted a relative data root")
	}
	if _, err := SubscriptionsDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("SubscriptionsDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := MihomoResolvedDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("MihomoResolvedDirectoryFromRoot() accepted a relative data root")
	}
	if _, err := MihomoProxyDelayCacheDirectoryFromRoot("jeemi_data"); err == nil {
		t.Fatal("MihomoProxyDelayCacheDirectoryFromRoot() accepted a relative data root")
	}
}
