package application

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/core"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

type authorizationDriver struct{ starts atomic.Int32 }

func (*authorizationDriver) Validate(context.Context, string, string, string) error { return nil }
func (d *authorizationDriver) Start(string, string, string) (mihomoruntime.Process, error) {
	d.starts.Add(1)
	return &fallbackTestProcess{done: make(chan struct{})}, nil
}

func authorizationService(t *testing.T, mode string) (*Service, *authorizationDriver) {
	t.Helper()
	return authorizationServiceWithProxy(t, mode, fallbackTestSystemProxy{})
}

func authorizationServiceWithProxy(t *testing.T, mode string, proxy mihomoruntime.SystemProxyController) (*Service, *authorizationDriver) {
	t.Helper()
	s, _, _ := fallbackService(t)
	s.runtime.Platform.OS = "linux"
	root := filepath.Dir(s.localConfigs.Directory())
	var err error
	s.versionManager, err = core.NewVersionManager(core.VersionManagerOptions{DataDirectory: root, GOOS: "windows", GOARCH: "amd64", Settings: s.settingsStore})
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic imported binary + fake driver: no execution, privileges or real networking.
	path := filepath.Join(t.TempDir(), "mihomo-v1.19.30.exe")
	if err := os.WriteFile(path, append([]byte("MZ"), make([]byte, 128)...), 0o700); err != nil {
		t.Fatal(err)
	}
	version, _, err := s.versionManager.Import(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.versionManager.Select(version); err != nil {
		t.Fatal(err)
	}
	driver := &authorizationDriver{}
	client := &http.Client{Transport: fallbackTestTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"version":"v1.19.30","mode":"rule","tun":{"enable":true,"device":"JeemiTun"}}`
		if r.Method == http.MethodGet && r.URL.Path == "/proxies" {
			body = `{"proxies":{}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{DataDirectory: root, Driver: driver, SystemProxy: proxy, HTTPClient: client, GOOS: "linux", ObserveTUN: func(context.Context, string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	preferences, err := s.RuntimePreferences()
	if err != nil {
		t.Fatal(err)
	}
	preferences.ProxyMode = mode
	if _, err := s.settingsStore.SetRuntimePreferences(preferences); err != nil {
		t.Fatal(err)
	}
	return s, driver
}

func TestBothProxyModesUseAuthorizationBeforeEveryExplicitStart(t *testing.T) {
	for _, mode := range []string{runtimeconfig.ProxyModeSystemProxy, runtimeconfig.ProxyModeTUN} {
		t.Run(string(mode), func(t *testing.T) {
			s, driver := authorizationService(t, mode)
			calls := 0
			s.authorizeCore = func(_ context.Context, target coreauth.Target) error {
				calls++
				if target.Size != 130 || len(target.SHA256) != 64 || target.ExecutablePath == "" {
					t.Fatalf("not the verified selection: %+v", target)
				}
				if driver.starts.Load() != int32(calls-1) {
					t.Fatal("core started before authorization")
				}
				return nil
			}
			for _, action := range []func() (RuntimeStatus, error){s.StartProxy, s.RestartProxy, s.ApplyGeoDataUpdates} {
				status, err := action()
				if err != nil || status.Mihomo.State != mihomoruntime.StateRunning || status.CoreAuthorizationPending {
					t.Fatalf("start: %+v %v", status, err)
				}
			}
			if calls != 3 || driver.starts.Load() != 3 {
				t.Fatalf("authorization/start mismatch: %d/%d", calls, driver.starts.Load())
			}
			preferences, _ := s.RuntimePreferences()
			preferences.OutboundMode = runtimeconfig.OutboundModeDirect
			if _, err := s.SaveRuntimePreferences(preferences); err != nil {
				t.Fatal(err)
			}
			if calls != 3 {
				t.Fatal("passive configuration change requested authentication")
			}
		})
	}
}

func TestAuthorizationFailureAndCancellationPreserveHealthyCore(t *testing.T) {
	for _, result := range []error{coreauth.ErrCancelled, &requirements.Error{Code: "linux_core_authentication_failed", Message: "authorization denied"}} {
		s, driver := authorizationService(t, runtimeconfig.ProxyModeTUN)
		s.authorizeCore = func(context.Context, coreauth.Target) error { return nil }
		before, err := s.StartProxy()
		if err != nil {
			t.Fatal(err)
		}
		s.authorizeCore = func(context.Context, coreauth.Target) error { return result }
		after, err := s.RestartProxy()
		if result == coreauth.ErrCancelled && err != nil {
			t.Fatal("cancellation reported as failure", err)
		}
		if result != coreauth.ErrCancelled && (err == nil || after.Configuration.LastError == nil || after.Configuration.LastError.Phase != "authorize") {
			t.Fatal("missing authorization failure", err)
		}
		if after.Mihomo.State != mihomoruntime.StateRunning || after.Mihomo.GenerationID != before.Mihomo.GenerationID || driver.starts.Load() != 1 || after.CoreAuthorizationPending {
			t.Fatalf("authorization changed healthy core: %+v", after)
		}
	}
}

func TestPendingAuthorizationCanBeCancelledStoppedOrQuit(t *testing.T) {
	for _, action := range []string{"cancel", "stop", "quit"} {
		t.Run(action, func(t *testing.T) {
			s, driver := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
			entered, finished := make(chan struct{}), make(chan error, 1)
			s.authorizeCore = func(ctx context.Context, _ coreauth.Target) error { close(entered); <-ctx.Done(); return ctx.Err() }
			go func() { _, err := s.StartProxy(); finished <- err }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("authorization did not start")
			}
			if !s.RuntimeStatus().CoreAuthorizationPending {
				t.Fatal("pending authorization was hidden")
			}
			duplicate := make(chan struct{})
			go func() { _, _ = s.RestartProxy(); close(duplicate) }()
			select {
			case <-duplicate:
			case <-time.After(2 * time.Second):
				t.Fatal("duplicate start was queued behind authentication")
			}
			switch action {
			case "cancel":
				s.CancelCoreAuthorization()
			case "stop":
				if _, err := s.StopProxy(); err != nil {
					t.Fatal(err)
				}
			case "quit":
				if err := s.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("authorization did not cancel")
			}
			if driver.starts.Load() != 0 || s.RuntimeStatus().CoreAuthorizationPending {
				t.Fatal("cancelled action started a core or left pending state")
			}
		})
	}
}
