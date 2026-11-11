package application

import (
	"context"
	"jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	"jeemi/internal/geodata"
	"jeemi/internal/runtimeconfig"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltInPresetsCreateBindAndPreserveEdits(t *testing.T) {
	s, driver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	s.runtime.Platform.OS = "linux"
	before, err := s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.StrategyGroups) != 0 || len(before.RuleSets) != 0 {
		t.Fatal("reading seeded preset resources")
	}
	general, err := s.CreateLocalConfigPreset("general")
	if err != nil {
		t.Fatal(err)
	}
	library, err := s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	edited := library.StrategyGroups[0]
	edited.Emoji = "🧠"
	edited.URL = "https://example.invalid/check"
	if _, err = s.SaveStrategyGroup(edited); err != nil {
		t.Fatal(err)
	}
	process, err := s.CreateLocalConfigPreset("process")
	if err != nil {
		t.Fatal(err)
	}
	library, err = s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	if len(library.StrategyGroups) != 7 || len(library.RuleSets) != 17 {
		t.Fatal("business groups or rules were duplicated", len(library.StrategyGroups), len(library.RuleSets))
	}
	found := false
	for _, g := range library.StrategyGroups {
		if g.ID == edited.ID {
			found = true
			if g.Emoji != edited.Emoji || g.URL != edited.URL {
				t.Fatal("preset overwrote user edits")
			}
		}
	}
	if !found {
		t.Fatal("edited group missing")
	}
	second, err := s.CreateLocalConfigPreset("general")
	if err != nil || second.ID == general.ID || second.Name == general.Name {
		t.Fatal("repeat creation must make a distinct editable configuration", err)
	}
	after, err := s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.StrategyGroups, library.StrategyGroups) || !reflect.DeepEqual(after.RuleSets, library.RuleSets) {
		t.Fatal("repeat creation rewrote shared resources")
	}
	subscriptions, err := s.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	id := subscriptions.SelectedSubscriptionID
	if _, err = s.SetSubscriptionLocalConfig(id, general.ID); err == nil || !strings.Contains(err.Error(), "GEO") {
		t.Fatal("general preset must require locally installed GEO databases", err)
	}
	installPresetTestGeo(t, s)
	if _, err = s.SetSubscriptionLocalConfig(id, general.ID); err != nil {
		t.Fatal(err)
	}
	baseline, err := s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(baseline.Contents, "MATCH,🚀 Jeemi") {
		t.Fatal("general preset was not composed", err)
	}
	s.runtime.Platform.OS = "unsupported"
	if _, err = s.SetSubscriptionLocalConfig(id, process.ID); err == nil {
		t.Fatal("unsupported platform changed binding")
	}
	state, err := s.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range state.Subscriptions {
		if sub.ID == id && sub.LocalConfigID != general.ID {
			t.Fatal("failed association was persisted")
		}
	}
	s.runtime.Platform.OS = "linux"
	if _, err = s.SetSubscriptionLocalConfig(id, process.ID); err != nil {
		t.Fatal(err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(view.Contents, "PROCESS-NAME,firefox,DIRECT") {
		t.Fatal("process rule missing", err)
	}
	packageBytes, _, err := s.ExportLocalPackage(configtransfer.ConfigKind, process.ID)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := configtransfer.Decode(packageBytes, configtransfer.ConfigKind)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Config.StrategyGroups) != 6 {
		t.Fatal("package lost groups")
	}
	autoCount := 0
	platformCount := 0
	for _, g := range pkg.Config.StrategyGroups {
		if g.AutoSelect {
			autoCount++
		}
	}
	for _, r := range pkg.Config.RuleSets {
		if r.Platform != "" {
			platformCount++
			if !strings.Contains(r.PayloadYAML, "PROCESS-NAME,") {
				t.Fatal("package lost editable browser process rules")
			}
		}
	}
	if autoCount != 4 || platformCount != 1 {
		t.Fatal("package lost preset options")
	}
	if driver.starts.Load() != 0 {
		t.Fatal("preset changes started the core")
	}
	if _, err = s.CreateLocalConfigPreset("unknown"); err == nil {
		t.Fatal("invalid preset accepted")
	}
}

func TestPresetResourceSwitchesValidatePersistAndRecompose(t *testing.T) {
	s, driver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	s.runtime.Platform.OS = "windows"
	preset, err := s.CreateLocalConfigPreset("process")
	if err != nil {
		t.Fatal(err)
	}
	subs, err := s.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetSubscriptionLocalConfig(subs.SelectedSubscriptionID, preset.ID); err != nil {
		t.Fatal(err)
	}
	library, err := s.LocalConfigResources()
	if err != nil {
		t.Fatal(err)
	}
	var instagram resources.RuleSet
	var video, jeemi resources.StrategyGroup
	for _, rule := range library.RuleSets {
		if strings.Contains(rule.Name, "Instagram") {
			instagram = rule
		}
	}
	for _, group := range library.StrategyGroups {
		if group.Name == "Video" {
			video = group
		}
		if group.Name == "Jeemi" {
			jeemi = group
		}
	}
	if instagram.ID == "" || video.ID == "" || jeemi.ID == "" {
		t.Fatal("missing categories")
	}
	instagram.Disabled = true
	if _, err = s.SaveRuleSet(instagram); err != nil {
		t.Fatal(err)
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || strings.Contains(view.Contents, "Instagram") || !strings.Contains(view.Contents, "🎬 Video Auto") {
		t.Fatal("rule switch removed wrong resources", err)
	}
	video.Disabled = true
	if _, err = s.SaveStrategyGroup(video); err != nil {
		t.Fatal(err)
	}
	view, err = s.RuntimeConfigurationText()
	if err != nil || strings.Contains(view.Contents, "🎬 Video") || strings.Contains(view.Contents, "Netflix") {
		t.Fatal("group switch did not remove selector, Auto and rules", err)
	}
	packageBytes, _, err := s.ExportLocalPackage(configtransfer.ConfigKind, preset.ID)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := configtransfer.Decode(packageBytes, configtransfer.ConfigKind)
	if err != nil {
		t.Fatal(err)
	}
	platforms := map[string]bool{}
	for _, rule := range pkg.Config.RuleSets {
		platforms[rule.Platform] = true
		if rule.ID == instagram.ID && (!rule.Disabled || rule.PayloadYAML != instagram.PayloadYAML) {
			t.Fatal("export lost disabled rule contents")
		}
	}
	for _, group := range pkg.Config.StrategyGroups {
		if group.ID == video.ID && (!group.Disabled || len(group.RuleSetReferences) != len(video.RuleSetReferences)) {
			t.Fatal("export lost disabled group references")
		}
	}
	for _, platform := range []string{"windows", "darwin", "linux"} {
		if platforms[platform] != (platform == "windows") {
			t.Fatal("export must contain only the created platform")
		}
	}
	before, _ := s.LocalConfigResources()
	jeemi.Disabled = true
	if _, err := s.SaveStrategyGroup(jeemi); err == nil {
		t.Fatal("disabled active MATCH target")
	}
	after, _ := s.LocalConfigResources()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed toggle changed library")
	}
	unchanged, err := s.RuntimeConfigurationText()
	if err != nil || unchanged.Contents != view.Contents {
		t.Fatal("failed toggle replaced valid configuration", err)
	}
	video.Disabled, instagram.Disabled = false, false
	if _, err = s.SaveStrategyGroup(video); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveRuleSet(instagram); err != nil {
		t.Fatal(err)
	}
	view, err = s.RuntimeConfigurationText()
	if err != nil || !strings.Contains(view.Contents, "Instagram") || !strings.Contains(view.Contents, "🎬 Video Auto") {
		t.Fatal("reenabling failed to restore routing", err)
	}
	if driver.starts.Load() != 0 {
		t.Fatal("resource switches started the core")
	}
}

type presetGeoValidator struct{}

func (presetGeoValidator) Validate(context.Context, string, geodata.Kind, string) error { return nil }

func installPresetTestGeo(t *testing.T, s *Service) {
	t.Helper()
	manager, err := geodata.NewManager(geodata.ManagerOptions{DataDirectory: filepath.Dir(s.localConfigs.Directory()), Validator: presetGeoValidator{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []geodata.Kind{geodata.KindGeoIPMMDB, geodata.KindGeoSite} {
		name, _ := geodata.CanonicalName(kind)
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte("synthetic GEO fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Import(context.Background(), kind, path, "fake-core", "v1", geodata.DefaultPreferences()); err != nil {
			t.Fatal(err)
		}
	}
	activation, err := manager.PrepareActivation(geodata.DefaultPreferences())
	if err != nil {
		t.Fatal(err)
	}
	if err = activation.Commit(); err != nil {
		t.Fatal(err)
	}
	s.geoDataManager = manager
}

func TestProcessPresetBindsWithoutGEO(t *testing.T) {
	for _, test := range []struct {
		goos    string
		browser string
	}{
		{"windows", "msedge.exe"},
		{"linux", "firefox"},
		{"darwin", "Google Chrome"},
	} {
		t.Run(test.goos, func(t *testing.T) {
			s, driver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
			s.runtime.Platform.OS = test.goos
			p, err := s.CreateLocalConfigPreset("process")
			if err != nil {
				t.Fatal(err)
			}
			state, err := s.SubscriptionState()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.SetSubscriptionLocalConfig(state.SelectedSubscriptionID, p.ID); err != nil {
				t.Fatal(err)
			}
			view, err := s.RuntimeConfigurationText()
			if err != nil {
				t.Fatal(err)
			}
			browserRule := "PROCESS-NAME," + test.browser + ",DIRECT"
			if strings.Contains(view.Contents, "GEOSITE,") || strings.Contains(view.Contents, "GEOIP,") || !strings.Contains(view.Contents, browserRule) {
				t.Fatal("process preset lost fixed browser routing or gained GEO dependency")
			}
			for _, browser := range []string{"msedge.exe", "firefox", "firefox-bin", "Google Chrome", "Safari", "com.apple.WebKit.Networking", "chrome.exe", "chrome"} {
				want := browser == test.browser || (test.goos == "linux" && browser == "firefox-bin")
				if strings.Contains(view.Contents, "PROCESS-NAME,"+browser+",DIRECT") != want {
					t.Fatalf("unexpected browser rule on %s: %s", test.goos, browser)
				}
			}
			chromeFrameworkRule := `PROCESS-PATH-REGEX,/Google Chrome\.app/,DIRECT`
			if strings.Contains(view.Contents, chromeFrameworkRule) != (test.goos == "darwin") {
				t.Fatal("Chrome app rule must only apply to macOS")
			}
			if strings.Index(view.Contents, ",🎬 Video") > strings.Index(view.Contents, browserRule) || strings.Index(view.Contents, browserRule) > strings.Index(view.Contents, "MATCH,🚀 Jeemi") {
				t.Fatal("browser rule must follow business routing and precede MATCH")
			}
			if driver.starts.Load() != 0 {
				t.Fatal("process preset binding started the core")
			}
		})
	}
}
