package application

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"jeemi/internal/externalui"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/runtimeconfig"
)

type uiDriver struct {
	authorizationDriver
	failNext atomic.Bool
}

func (d *uiDriver) Start(executable, home, configuration string) (mihomoruntime.Process, error) {
	if d.failNext.Swap(false) {
		return nil, errors.New("fixture launch failure")
	}
	return d.authorizationDriver.Start(executable, home, configuration)
}

func externalUIService(t *testing.T) (*Service, *uiDriver, *atomic.Int32) {
	t.Helper()
	s, _ := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	root := filepath.Dir(s.localConfigs.Directory())
	driver := &uiDriver{}
	client := &http.Client{Transport: fallbackTestTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"version":"v1.19.30","mode":"rule"}`
		if r.URL.Path == "/proxies" {
			body = `{"proxies":{}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	var err error
	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{DataDirectory: root, Driver: driver, SystemProxy: fallbackTestSystemProxy{}, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	s.authorizeCore = func(context.Context, coreauth.Target) error { return nil }
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, _ := writer.Create("dist/index.html")
	file.Write([]byte("fixture"))
	writer.Close()
	data := archive.Bytes()
	digest := sha256.Sum256(data)
	requests := &atomic.Int32{}
	s.zashboard = externalui.NewManager(externalui.Options{DataDirectory: root, HTTPClient: &http.Client{Transport: fallbackTestTransport(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		payload := data
		if r.URL.Host == "api.github.com" {
			version := "v3.26.0"
			if strings.Contains(r.URL.Path, "/tags/") {
				version = filepath.Base(r.URL.Path)
			}
			payload, _ = json.Marshal(map[string]any{"tag_name": version, "assets": []map[string]any{{"name": "dist.zip", "size": len(data), "digest": "sha256:" + hex.EncodeToString(digest[:]), "browser_download_url": externalui.ArchiveURL(version)}}})
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})}})
	return s, driver, requests
}

func TestExternalUIEnableDisableAndVersionSwitchRestartOnlyWhenRunning(t *testing.T) {
	s, driver, requests := externalUIService(t)
	if _, err := s.StartProxy(); err != nil {
		t.Fatal(err)
	}
	preferences, _ := s.RuntimePreferences()
	if preferences.ExternalUIEnabled || requests.Load() != 0 {
		t.Fatal("UI must default off and offline")
	}
	preferences.ExternalUIEnabled = true
	preferences, err := s.SaveRuntimePreferences(preferences)
	if err != nil || preferences.ExternalUIVersion != "v3.26.0" || requests.Load() != 2 || driver.starts.Load() != 2 {
		t.Fatal("enable did not download and restart", preferences.ExternalUIVersion, err, requests.Load(), driver.starts.Load())
	}
	if err := s.OpenZashboard(func(address string) {
		if !strings.Contains(address, "/ui/#/setup?") {
			t.Error("incorrect setup URL")
		}
	}); err != nil {
		t.Fatal(err)
	}
	preferences.LogLevel = "warning"
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 2 || requests.Load() != 2 {
		t.Fatal("ordinary hot reload downloaded UI or restarted")
	}
	if _, err := s.DownloadZashboardVersion("v3.25.0"); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 2 {
		t.Fatal("download applied an unconfirmed version")
	}
	if _, err := s.SelectZashboardVersion("v3.25.0"); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 3 {
		t.Fatal("selected version was hot reloaded instead of restarted")
	}
	preferences, _ = s.RuntimePreferences()
	preferences.ExternalUIEnabled = false
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 4 {
		t.Fatal("disabling must remove the existing HTTP route by restarting")
	}
	if err := s.OpenZashboard(func(string) { t.Error("disabled UI opened") }); err == nil {
		t.Fatal("disabled UI reported success")
	}
	view, err := s.RuntimeConfigurationText()
	if err != nil || strings.Contains(view.Contents, "external-ui") {
		t.Fatal("disabled UI survived config composition", err)
	}
	if _, err := s.StopProxy(); err != nil {
		t.Fatal(err)
	}
	preferences.ExternalUIEnabled = true
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 4 || s.runtimeManager.Status().State != mihomoruntime.StateStopped || requests.Load() != 4 {
		t.Fatal("enabling a stopped proxy started it or re-downloaded resources")
	}
}

func TestExternalUIFailedRestartRestoresPreviousHealthyGeneration(t *testing.T) {
	s, driver, _ := externalUIService(t)
	if _, err := s.StartProxy(); err != nil {
		t.Fatal(err)
	}
	before := s.runtimeManager.Status()
	driver.failNext.Store(true)
	preferences, _ := s.RuntimePreferences()
	preferences.ExternalUIEnabled = true
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	state := s.runtimeManager.Status()
	active, err := s.runtimeManager.ActiveStartRequest()
	if err != nil || state.State != mihomoruntime.StateRunning || active.Preferences.ExternalUIEnabled || state.SubscriptionID != before.SubscriptionID {
		t.Fatal("failed UI restart lost previous healthy request", err)
	}
	if s.runtimeConfigurationStatus().LastError == nil {
		t.Fatal("activation failure was hidden")
	}
	saved, _ := s.RuntimePreferences()
	if !saved.ExternalUIEnabled {
		t.Fatal("failed activation discarded user's saved intent")
	}
}

func TestExternalUIEmptySubscriptionAndClientRestartReuseLocalCache(t *testing.T) {
	s, driver, requests := externalUIService(t)
	preferences, _ := s.RuntimePreferences()
	preferences.ExternalUIEnabled = true
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || driver.starts.Load() != 0 {
		t.Fatal("preparing resources started proxy")
	}
	root := filepath.Dir(s.localConfigs.Directory())
	s.zashboard = externalui.NewManager(externalui.Options{DataDirectory: root, HTTPClient: &http.Client{Transport: fallbackTestTransport(func(*http.Request) (*http.Response, error) {
		t.Error("client restart re-downloaded UI")
		return nil, errors.New("offline")
	})}})
	if err := s.prepareExternalUI(context.Background()); err != nil {
		t.Fatal(err)
	}
	// No subscription is required to cache UI after toggling the Home switch.
	empty, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	empty.zashboard = s.zashboard
	preferences.ExternalUIVersion = ""
	saved, err := empty.SaveRuntimePreferences(preferences)
	if err != nil || saved.ExternalUIVersion != "v3.26.0" || empty.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("UI preparation incorrectly required a subscription", err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err != nil || !strings.Contains(string(contents), "externalUIVersion") {
		t.Fatal("version selection not persisted", err)
	}
}

func TestExternalUIDownloadFailureKeepsRunningCoreAndRetryAppliesDesiredUI(t *testing.T) {
	s, driver, _ := externalUIService(t)
	if _, err := s.StartProxy(); err != nil {
		t.Fatal(err)
	}
	before := s.runtimeManager.Status()
	workingDownloads := s.zashboard
	s.zashboard = externalui.NewManager(externalui.Options{DataDirectory: filepath.Dir(s.localConfigs.Directory()), HTTPClient: &http.Client{Transport: fallbackTestTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("fixture offline") })}})
	preferences, _ := s.RuntimePreferences()
	preferences.ExternalUIEnabled = true
	if _, err := s.SaveRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	after := s.runtimeManager.Status()
	if after.State != mihomoruntime.StateRunning || after.GenerationID != before.GenerationID || driver.starts.Load() != 1 {
		t.Fatal("download failure disturbed healthy core")
	}
	if failure := s.runtimeConfigurationStatus().LastError; failure == nil || failure.Code != "external_ui_prepare_failed" {
		t.Fatal("missing resource failure hidden")
	}
	s.zashboard = workingDownloads
	if _, err := s.DownloadZashboardVersion(""); err != nil {
		t.Fatal(err)
	}
	if driver.starts.Load() != 2 || s.runtimeConfigurationStatus().State != RuntimeConfigurationApplied {
		t.Fatal("repair did not retry the saved intent")
	}
}
