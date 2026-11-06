package application

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"jeemi/internal/geodata"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/runtimeconfig"
)

func TestDefaultFakeIPFilterRequiresGeoSiteWithoutGEORoutingRules(t *testing.T) {
	s, err := NewService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "plain.yaml")
	if err := os.WriteFile(path, []byte("rules: ['MATCH,DIRECT']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportSubscriptionFile(path); err != nil {
		t.Fatal(err)
	}
	if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthMissing || !reflect.DeepEqual(got.MissingKinds, []geodata.Kind{geodata.KindGeoSite}) {
		t.Fatalf("default DNS must require only GeoSite: %+v", got)
	}
	for _, test := range []struct {
		name    string
		enabled bool
		filter  []string
	}{
		{"disabled", false, runtimeconfig.DefaultPreferences().DNSFakeIPFilter},
		{"domain only", true, []string{"+.lan"}},
		{"empty", true, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			preferences := runtimeconfig.DefaultPreferences()
			preferences.DNSFakeIPFilterEnabled = test.enabled
			preferences.DNSFakeIPFilter = test.filter
			if _, err := s.SaveRuntimePreferences(preferences); err != nil {
				t.Fatal(err)
			}
			if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthReady {
				t.Fatalf("removed DNS reference still requires GEO: %+v", got)
			}
		})
	}
	installPresetTestGeo(t, s)
	defaults := runtimeconfig.DefaultPreferences()
	if _, err := s.SaveRuntimePreferences(defaults); err != nil {
		t.Fatal(err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil {
		t.Fatal(err)
	}
	var configuration struct {
		DNS struct {
			Filter []string `yaml:"fake-ip-filter"`
			Mode   string   `yaml:"fake-ip-filter-mode"`
		} `yaml:"dns"`
	}
	if err := yaml.Unmarshal([]byte(view.Contents), &configuration); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(configuration.DNS.Filter, defaults.DNSFakeIPFilter) || configuration.DNS.Mode != "blacklist" {
		t.Fatal("default filters or blacklist mode did not reach the final configuration")
	}
	if status := s.RuntimeStatus(); status.GeoData.Status != geodata.HealthReady || status.Mihomo.State != mihomoruntime.StateStopped {
		t.Fatalf("editing defaults started the core or reported unused GEO: %+v", status.GeoData)
	}
}

func importGeoHealthFixture(t *testing.T, s *Service) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "geo.yaml")
	if err := os.WriteFile(path, []byte("rules: ['GEOSITE,CN,DIRECT', 'MATCH,DIRECT']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return state.Subscriptions[len(state.Subscriptions)-1].ID
}

func TestRuntimeGEOHealthFollowsFinalCompositionAndSelection(t *testing.T) {
	s, _, first := fallbackService(t)
	if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthReady {
		t.Fatalf("plain configuration requires GEO: %+v", got)
	}
	id := importGeoHealthFixture(t, s)
	if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthReady {
		t.Fatalf("unselected subscription changed health: %+v", got)
	}
	if _, err := s.SelectSubscription(id); err != nil {
		t.Fatal(err)
	}
	if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthMissing || !reflect.DeepEqual(got.MissingKinds, []geodata.Kind{geodata.KindGeoSite}) {
		t.Fatalf("selected final configuration did not report only GeoSite: %+v", got)
	}
	if _, err := s.SelectSubscription(first.SelectedSubscriptionID); err != nil {
		t.Fatal(err)
	}
	if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthReady {
		t.Fatalf("old subscription requirements survived selection: %+v", got)
	}
	if _, err := s.SelectSubscription(id); err != nil {
		t.Fatal(err)
	}
	local, err := s.CreateLocalConfigPreset("process")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalConfig(id, local.ID); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if got := s.RuntimeStatus().GeoData; got.Status != geodata.HealthReady {
			t.Fatalf("source GEO survived local reconstruction: %+v", got)
		}
	}
	if state, err := s.GeoDataState(); err != nil || state.Health.Status != geodata.HealthMissing || len(state.Health.MissingKinds) != 3 {
		t.Fatalf("settings must continue reporting all missing assets: %+v, %v", state.Health, err)
	}
}

func TestRuntimeGEOHealthKeepsActiveRequirementsAcrossFailedSwitch(t *testing.T) {
	s, _ := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	s.authorizeCore = func(context.Context, coreauth.Target) error { return nil }
	if status, err := s.StartProxy(); err != nil || status.Mihomo.State != mihomoruntime.StateRunning || status.GeoData.Status != geodata.HealthReady {
		t.Fatalf("plain startup: state=%s health=%+v error=%v", status.Mihomo.State, status.GeoData, err)
	}
	id := importGeoHealthFixture(t, s)
	if _, err := s.SelectSubscription(id); err != nil {
		t.Fatal(err)
	}
	status := s.RuntimeStatus()
	if status.Configuration.LastError == nil || status.Mihomo.State != mihomoruntime.StateRunning || status.GeoData.Status != geodata.HealthReady {
		t.Fatalf("failed candidate replaced active GEO requirements: health=%+v configuration=%+v", status.GeoData, status.Configuration.LastError)
	}
	status, err := s.StopProxy()
	if err != nil || status.GeoData.Status != geodata.HealthMissing {
		t.Fatalf("stopped runtime did not use desired requirements: %+v, %v", status.GeoData, err)
	}
	installPresetTestGeo(t, s)
	status, err = s.StartProxy()
	if err != nil || status.GeoData.Status != geodata.HealthReady {
		t.Fatalf("missing unused ASN blocked GeoSite-only startup: %+v, %v", status.GeoData, err)
	}
	activeSite := filepath.Join(filepath.Dir(s.localConfigs.Directory()), "runtime", "mihomo", "GeoSite.dat")
	if err := os.Remove(activeSite); err != nil {
		t.Fatal(err)
	}
	status = s.RuntimeStatus()
	if status.GeoData.Status != geodata.HealthInvalid || !reflect.DeepEqual(status.GeoData.InvalidKinds, []geodata.Kind{geodata.KindGeoSite}) || status.Mihomo.State != mihomoruntime.StateRunning {
		t.Fatalf("active required asset damage was hidden or stopped the core: %+v", status.GeoData)
	}
}
