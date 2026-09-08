package application

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/core"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/settings"
)

type fallbackTestProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fallbackTestProcess) PID() int              { return 1001 }
func (p *fallbackTestProcess) Done() <-chan struct{} { return p.done }
func (p *fallbackTestProcess) ExitError() error      { return nil }
func (p *fallbackTestProcess) Stop(context.Context) error {
	p.once.Do(func() { close(p.done) })
	return nil
}

type fallbackTestDriver struct{ validationErr error }

func (d *fallbackTestDriver) Validate(context.Context, string, string, string) error {
	return d.validationErr
}
func (d *fallbackTestDriver) Start(string, string, string) (mihomoruntime.Process, error) {
	return &fallbackTestProcess{done: make(chan struct{})}, nil
}

type fallbackTestSystemProxy struct{}

func (fallbackTestSystemProxy) Apply(context.Context, runtimeconfig.Preferences) error { return nil }
func (fallbackTestSystemProxy) Restore(context.Context) error                          { return nil }
func (fallbackTestSystemProxy) Recover(context.Context) error                          { return nil }

type fallbackTestTransport func(*http.Request) (*http.Response, error)

func (f fallbackTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFallbackSwitchResetsConnectionsOnlyAfterAppliedUsingSavedPreference(t *testing.T) {
	for _, test := range []struct {
		name, mode                             string
		captureFailure, deleteFailure, invalid bool
		closed                                 []string
	}{
		{name: "off", mode: settings.ConnectionResetOff},
		{name: "selector", mode: settings.ConnectionResetSelector, closed: []string{"/connections/original"}},
		{name: "all", mode: settings.ConnectionResetAll, closed: []string{"/connections/direct", "/connections/original", "/connections/other"}},
		{name: "capture failure", mode: settings.ConnectionResetSelector, captureFailure: true},
		{name: "delete failure", mode: settings.ConnectionResetSelector, deleteFailure: true, closed: []string{"/connections/original"}},
		{name: "validation failure", mode: settings.ConnectionResetSelector, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, state := fallbackService(t)
			s.authorizeCore = func(context.Context, coreauth.Target) error { return nil }
			root := filepath.Dir(s.localConfigs.Directory())
			var err error
			s.versionManager, err = core.NewVersionManager(core.VersionManagerOptions{DataDirectory: root, GOOS: "windows", GOARCH: "amd64", Settings: s.settingsStore})
			if err != nil {
				t.Fatal(err)
			}
			// Import a synthetic header into the temporary test root. The injected
			// driver never executes it or changes real proxy/TUN state.
			executable := filepath.Join(t.TempDir(), "mihomo-v1.19.30.exe")
			if err := os.WriteFile(executable, append([]byte("MZ"), make([]byte, 128)...), 0o700); err != nil {
				t.Fatal(err)
			}
			version, _, err := s.versionManager.Import(context.Background(), executable)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.versionManager.Select(version); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var closed []string
			captures := 0
			client := &http.Client{Transport: fallbackTestTransport(func(r *http.Request) (*http.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				status, body := http.StatusNoContent, ""
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/version":
					status, body = http.StatusOK, `{"version":"v1.19.30"}`
				case r.Method == http.MethodGet && r.URL.Path == "/configs":
					status, body = http.StatusOK, `{"mode":"rule"}`
				case r.Method == http.MethodGet && r.URL.Path == "/proxies":
					status, body = http.StatusOK, `{"proxies":{}}`
				case r.Method == http.MethodGet && r.URL.Path == "/connections":
					if s.runtimeConfigurationStatus().State != RuntimeConfigurationApplied {
						t.Error("connection snapshot was read before successful application")
					}
					captures++
					status, body = http.StatusOK, `{"connections":[{"id":"original","chains":["Original"]},{"id":"direct","chains":["DIRECT"]},{"id":"other","chains":["Hidden"]}]}`
					if test.captureFailure {
						status = http.StatusServiceUnavailable
					}
				case r.Method == http.MethodDelete:
					if s.runtimeConfigurationStatus().State != RuntimeConfigurationApplied {
						t.Error("connection reset occurred before coordinator reported successful application")
					}
					closed = append(closed, r.URL.Path)
					if test.deleteFailure {
						status = http.StatusServiceUnavailable
					}
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			driver := &fallbackTestDriver{}
			s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{
				DataDirectory: root, Driver: driver, SystemProxy: fallbackTestSystemProxy{}, HTTPClient: client, GOOS: "windows",
				// The default mode is TUN, but the fake driver cannot create a real
				// interface. Keep readiness independent of the host's network state.
				ObserveTUN: func(context.Context, string) error { return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = s.runtimeManager.Stop(context.Background()) })
			if _, err := s.settingsStore.SetSubscriptionPreferences(settings.DefaultSelectorDensity, settings.DefaultDelayTestConcurrency, test.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := s.StartProxy(); err != nil {
				t.Fatal(err)
			}
			if test.invalid {
				driver.validationErr = context.Canceled
			}
			result, err := s.SetSubscriptionFallback(state.SelectedSubscriptionID, fallbackoverride.Selection{Mode: fallbackoverride.ModeSelector, Selector: "Hidden"}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if result.State.Projection.Fallback.Selection.Selector != "Hidden" || result.ConnectionResetFailed != (test.captureFailure || test.deleteFailure) {
				t.Fatalf("saved state and reset outcome were not independent: %+v", result)
			}
			mu.Lock()
			defer mu.Unlock()
			slices.Sort(closed)
			if !slices.Equal(closed, test.closed) || ((test.invalid || test.mode == settings.ConnectionResetOff) && captures != 0) {
				t.Fatalf("preference %s: captured %d, closed %v, expected %v", test.mode, captures, closed, test.closed)
			}
			active := s.runtimeManager.Status()
			if test.invalid && active.Source.FallbackOverrideRevision != 0 {
				t.Fatal("failed validation replaced the active fallback")
			}
			if !test.invalid && active.Source.FallbackOverrideRevision != 1 {
				t.Fatal("connection reset error prevented fallback activation")
			}
		})
	}
}
