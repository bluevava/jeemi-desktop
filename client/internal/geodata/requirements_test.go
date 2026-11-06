package geodata

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRequiredKindsInspectOnlyGEOConsumers(t *testing.T) {
	for _, tc := range []struct {
		name, configuration string
		want                []Kind
	}{
		{"plain rules", "rules: ['PROCESS-NAME,chrome.exe,DIRECT']\n", nil},
		{"metadata and targets", "# GEOSITE,CN\nproxies: [{name: 'GEOIP,CN', password: 'GEOSITE:secret'}]\nproxy-groups: [{name: 'IP-ASN,13335'}]\nrules: ['DOMAIN-KEYWORD,GEOSITE:label,DIRECT', 'MATCH,GEOSITE:label']\n", nil},
		{"country", "rules: ['GEOIP,CN,DIRECT']", []Kind{KindGeoIPMMDB}},
		{"site", "rules: ['GEOSITE,CN,DIRECT']", []Kind{KindGeoSite}},
		{"asn", "rules: ['IP-ASN,13335,DIRECT', 'SRC-IP-ASN,9808,DIRECT']", []Kind{KindASN}},
		{"logical and source", "rules: ['AND,((SRC-GEOIP,CN),(OR,((GEOSITE,cn),(IP-ASN,13335)))),DIRECT']", []Kind{KindGeoIPMMDB, KindGeoSite, KindASN}},
		{"sub rules", "rules: ['SUB-RULE,(GEOIP,CN),region']\nsub-rules: {region: ['GEOSITE,cn,DIRECT']}", []Kind{KindGeoIPMMDB, KindGeoSite}},
		{"inline provider", "rule-providers: {Local: {type: inline, behavior: classical, payload: ['GEOSITE,cn']}}\nrules: ['RULE-SET,Local,DIRECT']", []Kind{KindGeoSite}},
		{"domain provider is literal", "rule-providers: {Local: {type: inline, behavior: domain, payload: ['geosite:example']}}\nrules: ['RULE-SET,Local,DIRECT']", nil},
		{"remote URL is not a rule", "rule-providers: {Local: {type: http, behavior: classical, url: 'https://example.test/GEOIP:cn'}}", nil},
		{"DNS policy", "dns: {enable: true, nameserver-policy: {'geosite:cn': '223.5.5.5'}}", []Kind{KindGeoSite}},
		{"DNS server URL", "dns: {enable: true, nameserver: ['https://example.test/GEOSITE:cn']}", nil},
		{"disabled DNS", "dns: {enable: false, nameserver-policy: {'geosite:cn': '223.5.5.5'}, fallback: ['1.1.1.1']}", nil},
		{"unused proxy DNS policy", "dns: {enable: true, proxy-server-nameserver-policy: {'geosite:cn': '223.5.5.5'}}", nil},
		{"proxy DNS policy", "dns: {enable: true, proxy-server-nameserver: ['1.1.1.1'], proxy-server-nameserver-policy: {'geosite:cn': '223.5.5.5'}}", []Kind{KindGeoSite}},
		{"fake IP domain set", "dns: {enable: true, fake-ip-filter: ['geosite:cn']}", []Kind{KindGeoSite}},
		{"fake IP rule", "dns: {enable: true, fake-ip-filter-mode: rule, fake-ip-filter: ['GEOSITE,cn,real-ip']}", []Kind{KindGeoSite}},
		{"fallback default", "dns: {enable: true, fallback: ['1.1.1.1']}", []Kind{KindGeoIPMMDB}},
		{"fallback site only", "dns: {enable: true, fallback: ['1.1.1.1'], fallback-filter: {geoip: false, geosite: [gfw]}}", []Kind{KindGeoSite}},
		{"unused fallback filter", "dns: {enable: true, fallback-filter: {geoip: true, geosite: [gfw]}}", nil},
		{"aliased rules", "shared: &rules ['GEOSITE,cn,DIRECT']\nrules: *rules", []Kind{KindGeoSite}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RequiredKinds([]byte(tc.configuration), DefaultPreferences())
			if err != nil || !reflect.DeepEqual(got, append([]Kind{}, tc.want...)) {
				t.Fatalf("requirements = %v, error = %v; want %v", got, err, tc.want)
			}
		})
	}
	preferences := DefaultPreferences()
	preferences.GeoIPMode = GeoIPModeDAT
	got, err := RequiredKinds([]byte("rules: ['GEOIP,CN,DIRECT']"), preferences)
	if err != nil || !reflect.DeepEqual(got, []Kind{KindGeoIPDAT}) {
		t.Fatalf("DAT requirement = %v, %v", got, err)
	}
	deepRule := strings.Repeat("NOT,((", 70) + "GEOSITE,cn" + strings.Repeat("))", 70)
	if _, err := RequiredKinds([]byte("rules: ['"+deepRule+",DIRECT']"), preferences); err == nil {
		t.Fatal("unbounded logical nesting was accepted")
	}
}

func TestRequiredHealthIgnoresUnreferencedMissingAndInvalidAssets(t *testing.T) {
	manager, err := NewManager(ManagerOptions{DataDirectory: t.TempDir(), Validator: &acceptingValidator{}})
	if err != nil {
		t.Fatal(err)
	}
	preferences := DefaultPreferences()
	assertHealth := func(kinds []Kind, want string) Health {
		t.Helper()
		health, err := manager.HealthForKinds(preferences, kinds)
		if err != nil || health.Status != want {
			t.Fatalf("health for %v = %+v, %v; want %s", kinds, health, err, want)
		}
		return health
	}
	assertHealth(nil, HealthReady)
	missing := assertHealth([]Kind{KindGeoSite}, HealthMissing)
	if !reflect.DeepEqual(missing.MissingKinds, []Kind{KindGeoSite}) {
		t.Fatalf("unreferenced missing assets leaked into health: %+v", missing)
	}
	source := filepath.Join(t.TempDir(), "fixture.dat")
	if err := os.WriteFile(source, []byte("synthetic GEO fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Import(context.Background(), KindGeoSite, source, "fake", "v1", preferences); err != nil {
		t.Fatal(err)
	}
	assertHealth([]Kind{KindGeoSite}, HealthPending)
	activation, err := manager.PrepareActivation(preferences)
	if err != nil {
		t.Fatal(err)
	}
	if err := activation.Commit(); err != nil {
		t.Fatal(err)
	}
	assertHealth([]Kind{KindGeoSite}, HealthReady)
	if full, err := manager.Health(preferences); err != nil || full.Status != HealthMissing || len(full.MissingKinds) != 2 {
		t.Fatalf("settings no longer reports all assets: %+v, %v", full, err)
	}
	state, err := manager.Import(context.Background(), KindGeoIPMMDB, source, "fake", "v1", preferences)
	if err != nil {
		t.Fatal(err)
	}
	assertHealth([]Kind{KindGeoSite}, HealthReady) // unrelated pending revision
	for _, asset := range state.Assets {
		if asset.Kind == KindGeoIPMMDB {
			if err := os.WriteFile(manager.revisionAssetPath(asset.Kind, asset.SHA256), []byte("damaged unrelated database"), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertHealth([]Kind{KindGeoSite}, HealthReady)
	if full, err := manager.Health(preferences); err != nil || full.Status != HealthInvalid {
		t.Fatalf("settings hid an unrelated damaged asset: %+v, %v", full, err)
	}
	if err := os.WriteFile(filepath.Join(manager.runtimeDirectory, "GeoSite.dat"), []byte("damaged active database"), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := assertHealth([]Kind{KindGeoSite}, HealthInvalid)
	if !reflect.DeepEqual(invalid.InvalidKinds, []Kind{KindGeoSite}) {
		t.Fatalf("wrong invalid asset: %+v", invalid)
	}
	assertHealth(nil, HealthReady)
	// A configuration with no GEO use must not diagnose an unrelated manifest.
	if err := os.WriteFile(filepath.Join(manager.directory, "manifest.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	assertHealth(nil, HealthReady)
}
