package runtime

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jeemi/internal/config/runtimeoverride"
	"jeemi/internal/runtimeconfig"
)

type fakeProcess struct {
	pid  int
	done chan struct{}
	once sync.Once
	err  error
}

func newFakeProcess(pid int) *fakeProcess         { return &fakeProcess{pid: pid, done: make(chan struct{})} }
func (p *fakeProcess) PID() int                   { return p.pid }
func (p *fakeProcess) Done() <-chan struct{}      { return p.done }
func (p *fakeProcess) ExitError() error           { return p.err }
func (p *fakeProcess) Stop(context.Context) error { p.once.Do(func() { close(p.done) }); return nil }
func (p *fakeProcess) crash(err error)            { p.err = err; p.once.Do(func() { close(p.done) }) }

type fakeRuntimeDriver struct {
	mu          sync.Mutex
	starts      []*fakeProcess
	validations int
	validateErr error
}

func (d *fakeRuntimeDriver) Validate(context.Context, string, string, string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.validations++
	return d.validateErr
}
func (d *fakeRuntimeDriver) Start(string, string, string) (Process, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	process := newFakeProcess(1000 + len(d.starts) + 1)
	d.starts = append(d.starts, process)
	return process, nil
}

type fakeSystemProxy struct {
	mu         sync.Mutex
	applied    int
	restored   int
	recoverErr error
}

func (p *fakeSystemProxy) Apply(context.Context, runtimeconfig.Preferences) error {
	p.mu.Lock()
	p.applied++
	p.mu.Unlock()
	return nil
}
func (p *fakeSystemProxy) Restore(context.Context) error {
	p.mu.Lock()
	p.restored++
	p.mu.Unlock()
	return nil
}
func (p *fakeSystemProxy) Recover(context.Context) error { return p.recoverErr }

type transportFunc func(*http.Request) (*http.Response, error)

func (function transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func healthyHTTPClient() *http.Client {
	return &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		status := http.StatusNoContent
		if request.Method == http.MethodGet && request.URL.Path == "/version" {
			status = http.StatusOK
			body = `{"version":"v1.19.30"}`
		}
		if request.Method == http.MethodGet && request.URL.Path == "/configs" {
			status = http.StatusOK
			body = `{"mode":"rule"}`
		}
		if request.Method == http.MethodGet && request.URL.Path == "/proxies" {
			status = http.StatusOK
			body = `{"proxies":{"Main":{"type":"Selector","now":"Node A","all":["Node A","Node B"]}}}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

func TestManagerStartsStopsAndRecoversDirectChildProcess(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	proxy := &fakeSystemProxy{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: proxy,
		HTTPClient: healthyHTTPClient(), GOOS: "windows", RestartDelays: []time.Duration{time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	status, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateRunning || status.PID != 1001 || !status.SystemProxy || !status.ControllerReady || status.ControllerSession == nil {
		t.Fatalf("start status = %+v", status)
	}
	if proxy.applied != 1 {
		t.Fatalf("system proxy apply count = %d", proxy.applied)
	}
	view, err := manager.ActiveConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	if view.GenerationID != status.GenerationID || strings.Contains(string(view.Contents), "external-controller") || strings.Contains(string(view.Contents), "secret:") || strings.Contains(string(view.Contents), "external-controller-cors") {
		t.Fatalf("active configuration view exposed session fields: %+v\n%s", view, view.Contents)
	}
	if err := manager.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	if status = manager.Status(); status.OutboundMode != "global" || manager.active.Preferences.OutboundMode != "global" {
		t.Fatalf("dynamic mode was not retained for recovery: %+v", status)
	}
	if err := manager.SelectProxy(context.Background(), "Main", "Node A"); err != nil {
		t.Fatal(err)
	}

	driver.starts[0].crash(context.Canceled)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status = manager.Status()
		if status.State == StateRunning && status.PID == 1002 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if status.State != StateRunning || status.PID != 1002 || status.RestartAttempt != 1 || status.OutboundMode != "global" {
		t.Fatalf("recovery status = %+v", status)
	}
	configPath := manager.active.ConfigPath
	status, err = manager.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateStopped || status.DesiredRunning {
		t.Fatalf("stop status = %+v", status)
	}
	if proxy.restored < 2 {
		t.Fatalf("system proxy restore count = %d", proxy.restored)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("stopped runtime retained session configuration: %v", err)
	}
	if status, err = manager.Stop(context.Background()); err != nil || status.State != StateStopped {
		t.Fatalf("repeated stop must be idempotent: status=%+v err=%v", status, err)
	}
}

func TestManagerValidatesResolvedSnapshotWithoutStartingOrChangingRuntimeState(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
		SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "windows",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateConfiguration(context.Background(), `C:\jeemi\mihomo.exe`, []byte("mode: rule\nproxies: []\n")); err != nil {
		t.Fatal(err)
	}
	if driver.validations != 1 || len(driver.starts) != 0 {
		t.Fatalf("validation count=%d process starts=%d", driver.validations, len(driver.starts))
	}
	if status := manager.Status(); status.State != StateStopped || status.DesiredRunning {
		t.Fatalf("validation changed runtime state: %+v", status)
	}
}

func TestManagerEnablesAndObservesJeemiTunOnlyAfterControllerHealth(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	proxy := &fakeSystemProxy{}
	var observed string
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: proxy,
		HTTPClient: healthyHTTPClient(), GOOS: "windows",
		ObserveTUN: func(_ context.Context, name string) error { observed = name; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeTUN))
	if err != nil {
		t.Fatal(err)
	}
	if !status.TUNEnabled || status.TUNDevice != "JeemiTun" || observed != "JeemiTun" {
		t.Fatalf("TUN status=%+v observed=%q", status, observed)
	}
	if proxy.applied != 0 {
		t.Fatal("TUN start modified the system proxy")
	}
	if _, err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartupRecoveryFailureBlocksProxyStart(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	proxy := &fakeSystemProxy{recoverErr: context.DeadlineExceeded}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
		SystemProxy: proxy, HTTPClient: healthyHTTPClient(), GOOS: "windows",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverSystemProxy(context.Background()); err == nil {
		t.Fatal("startup recovery unexpectedly succeeded")
	}
	status, err := manager.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy))
	if err == nil || status.State != StateFailed || status.LastError == nil || status.LastError.Code != "startup_recovery_failed" {
		t.Fatalf("blocked start status=%+v err=%v", status, err)
	}
	if len(driver.starts) != 0 {
		t.Fatalf("blocked startup launched %d processes", len(driver.starts))
	}
}

func TestFailedReloadKeepsActiveGenerationSource(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
		SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "windows",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	started, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	driver.mu.Lock()
	driver.validateErr = context.Canceled
	driver.mu.Unlock()
	candidate := request
	candidate.Source.SubscriptionID = strings.Repeat("c", 32)
	status, err := manager.Start(context.Background(), candidate)
	if err == nil {
		t.Fatal("reload unexpectedly succeeded")
	}
	if status.State != StateRunning || status.GenerationID != started.GenerationID || status.Source.SubscriptionID != request.Source.SubscriptionID {
		t.Fatalf("failed reload replaced active source: %+v", status)
	}
	if _, err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManagerHotReloadsWithoutReplacingProcess(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
		SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "windows",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	started, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	candidate := request
	candidate.Configuration = append(append([]byte{}, request.Configuration...), []byte("unified-delay: true\n")...)
	candidate.Source.SubscriptionRevision = strings.Repeat("c", 64)
	reloaded, err := manager.Start(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.State != StateRunning || reloaded.PID != started.PID || reloaded.GenerationID == started.GenerationID {
		t.Fatalf("hot reload status: started=%+v reloaded=%+v", started, reloaded)
	}
	if reloaded.Source.SubscriptionRevision != candidate.Source.SubscriptionRevision {
		t.Fatalf("hot reload retained stale source: %+v", reloaded.Source)
	}
	if len(driver.starts) != 1 {
		t.Fatalf("hot reload launched %d processes", len(driver.starts))
	}
	if _, err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestManualStartRetiresCrashedControllerSessionBeforeRecovery(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
		SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "windows",
		RestartDelays: []time.Duration{time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := manager.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	oldConfigPath := manager.active.ConfigPath
	driver.starts[0].crash(context.Canceled)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.Status().State == StateRecovering {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if status := manager.Status(); status.State != StateRecovering {
		t.Fatalf("crashed runtime did not enter recovery: %+v", status)
	}
	candidate := request
	candidate.Source.SubscriptionRevision = strings.Repeat("d", 64)
	status, err := manager.Start(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateRunning || status.PID != 1002 || len(driver.starts) != 2 {
		t.Fatalf("manual recovery status=%+v starts=%d", status, len(driver.starts))
	}
	if _, err := os.Stat(oldConfigPath); !os.IsNotExist(err) {
		t.Fatalf("manual recovery retained the crashed session configuration: %v", err)
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
}

func TestManagerStopsAfterBoundedRapidCrashRecovery(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: &fakeSystemProxy{},
		HTTPClient: healthyHTTPClient(), GOOS: "windows",
		RestartDelays: []time.Duration{time.Millisecond, time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err != nil {
		t.Fatal(err)
	}
	for expectedPID := 1002; expectedPID <= 1003; expectedPID++ {
		driver.starts[expectedPID-1002].crash(context.Canceled)
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if status := manager.Status(); status.State == StateRunning && status.PID == expectedPID {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if status := manager.Status(); status.PID != expectedPID || status.RestartAttempt != expectedPID-1001 {
			t.Fatalf("recovery %d status = %+v", expectedPID, status)
		}
	}
	driver.starts[2].crash(context.Canceled)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.Status().State == StateFailed {
			break
		}
		time.Sleep(time.Millisecond)
	}
	status := manager.Status()
	if status.State != StateFailed || status.DesiredRunning || status.LastError == nil || status.LastError.Code != "process_exited" {
		t.Fatalf("bounded recovery status = %+v", status)
	}
}

func runtimeRequest(t *testing.T, proxyMode string) StartRequest {
	t.Helper()
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ProxyMode = proxyMode
	configuration, err := runtimeoverride.ApplyForPlatform([]byte("proxies: []\nrules: [MATCH,DIRECT]\n"), preferences, "windows")
	if err != nil {
		t.Fatal(err)
	}
	return StartRequest{
		ExecutablePath: `C:\jeemi\mihomo.exe`, CoreVersion: "v1.19.30", Configuration: configuration,
		Preferences: preferences, Source: Source{SubscriptionID: strings.Repeat("a", 32), SubscriptionRevision: strings.Repeat("b", 64)},
	}
}
