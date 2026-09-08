package resolved

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"jeemi/internal/geodata"
	"jeemi/internal/runtimeconfig"
)

func testGeoDataInput() (geodata.Preferences, string) {
	return geodata.DefaultPreferences(), strings.Repeat("d", 64)
}

func TestStorePersistsSafePerSubscriptionSnapshot(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	geoPreferences, geoFingerprint := testGeoDataInput()
	input := Input{
		SubscriptionID: strings.Repeat("a", 32), SubscriptionRevision: strings.Repeat("b", 64),
		LocalConfigID: strings.Repeat("c", 32), LocalConfigRevision: 3,
		CoreVersion: "v1.19.30", CoreValidated: true,
		Preferences:        runtimeconfig.DefaultPreferences(),
		GeoDataPreferences: geoPreferences, GeoDataFingerprint: geoFingerprint,
		Configuration: []byte("external-controller: 0.0.0.0:9090\nsecret: leaked\nmode: rule\nproxies: []\n"),
	}
	saved, err := store.Save(input)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved.Configuration, []byte("leaked")) || bytes.Contains(saved.Configuration, []byte("external-controller")) {
		t.Fatalf("resolved snapshot retained session fields:\n%s", saved.Configuration)
	}
	loaded, err := store.Current(input.SubscriptionID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Fingerprint != saved.Manifest.Fingerprint || !bytes.Equal(loaded.Configuration, saved.Configuration) {
		t.Fatalf("loaded snapshot differs: saved=%+v loaded=%+v", saved.Manifest, loaded.Manifest)
	}
	info, err := os.Stat(filepath.Join(saved.Directory, "current.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("resolved configuration permissions are too broad: %v", info.Mode().Perm())
	}
}

func TestFingerprintChangesWithSourceOrRuntimePreferences(t *testing.T) {
	geoPreferences, geoFingerprint := testGeoDataInput()
	base := Input{
		SubscriptionID: strings.Repeat("a", 32), SubscriptionRevision: strings.Repeat("b", 64),
		Preferences: runtimeconfig.DefaultPreferences(), GeoDataPreferences: geoPreferences,
		GeoDataFingerprint: geoFingerprint, Configuration: []byte("mode: rule\n"),
	}
	first, _, err := Fingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.Preferences.ListenPort++
	second, _, err := Fingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("runtime preference change did not change the resolved fingerprint")
	}
	changed = base
	changed.RuleProviderOverrideRevision = 1
	third, _, err := Fingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == third {
		t.Fatal("rule provider override revision did not change the resolved fingerprint")
	}
	changed = base
	changed.FallbackOverrideRevision = 1
	fallbackFingerprint, _, err := Fingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == fallbackFingerprint {
		t.Fatal("fallback override revision did not change the resolved fingerprint")
	}
	changed = base
	changed.GeoDataFingerprint = strings.Repeat("e", 64)
	fourth, _, err := Fingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first == fourth {
		t.Fatal("GEO data revision change did not change the resolved fingerprint")
	}
	changed = base
	changed.GeoDataPreferences.Source = geodata.SourceCustom
	changed.GeoDataPreferences.CustomURLs.GeoSite = "https://example.test/geosite.dat"
	fifth, _, err := Fingerprint(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first != fifth {
		t.Fatal("download-only GEO source change altered the runtime fingerprint")
	}
}

func TestStoreReadsPreviousCompleteSnapshotAfterInterruptedDirectorySwap(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	geoPreferences, geoFingerprint := testGeoDataInput()
	input := Input{
		SubscriptionID: strings.Repeat("a", 32), SubscriptionRevision: strings.Repeat("b", 64),
		Preferences: runtimeconfig.DefaultPreferences(), GeoDataPreferences: geoPreferences,
		GeoDataFingerprint: geoFingerprint, Configuration: []byte("mode: rule\nproxies: []\n"),
	}
	saved, err := store.Save(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved.Directory, saved.Directory+".previous"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Current(input.SubscriptionID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Fingerprint != saved.Manifest.Fingerprint || !bytes.Equal(loaded.Configuration, saved.Configuration) {
		t.Fatalf("fallback snapshot differs: saved=%+v loaded=%+v", saved.Manifest, loaded.Manifest)
	}
}
