package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

type checkedRuntimeDriver struct {
	fakeRuntimeDriver
	fail bool
	pid  int
}

func (d *checkedRuntimeDriver) Preflight(_ string, _ runtimeconfig.Preferences, pid int) error {
	d.pid = pid
	if d.fail {
		return &requirements.Error{Code: "linux_core_permission_required", Message: "missing permissions"}
	}
	return nil
}

func TestPlatformPreflightRetainsHealthyGenerationAndChecksRestartedProcess(t *testing.T) {
	driver := &checkedRuntimeDriver{}
	proxy := &fakeSystemProxy{}
	manager, err := NewManager(Options{DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: proxy, HTTPClient: healthyHTTPClient(), GOOS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(context.Background())
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	before, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	driver.fail = true
	request.Preferences.ProxyMode = runtimeconfig.ProxyModeTUN
	after, err := manager.Start(context.Background(), request)
	if err == nil || after.State != StateRunning || after.GenerationID != before.GenerationID || after.PID != before.PID || after.LastError.Code != "linux_core_permission_required" {
		t.Fatalf("healthy runtime lost on preflight failure: %+v %v", after, err)
	}
	if driver.pid != before.PID || proxy.restored != 0 || len(driver.starts) != 1 {
		t.Fatal("preflight changed the live process/network")
	}
	_ = manager.CheckPrerequisites(context.Background(), request, true)
	if driver.pid != 0 {
		t.Fatal("full restart checked old process permissions")
	}
}

func TestRecoveryDoesNotStartUnprivilegedReplacementInEitherProxyMode(t *testing.T) {
	for _, mode := range []string{runtimeconfig.ProxyModeSystemProxy, runtimeconfig.ProxyModeTUN} {
		t.Run(mode, func(t *testing.T) {
			driver := &checkedRuntimeDriver{}
			manager, err := NewManager(Options{
				DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver,
				SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "linux",
				RestartDelays: []time.Duration{time.Millisecond}, ObserveTUN: func(context.Context, string) error { return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Shutdown(context.Background())
			if _, err := manager.Start(context.Background(), runtimeRequest(t, mode)); err != nil {
				t.Fatal(err)
			}
			driver.fail = true // File capabilities disappear before the original process exits.
			driver.starts[0].crash(errors.New("simulated crash"))
			deadline := time.After(2 * time.Second)
			for {
				status := manager.Status()
				if status.State == StateFailed {
					if status.LastError == nil || status.LastError.Code != "linux_core_permission_required" || status.DesiredRunning {
						t.Fatalf("wrong recovery state: %+v", status)
					}
					break
				}
				select {
				case <-deadline:
					t.Fatal("recovery did not stop")
				case <-time.After(time.Millisecond):
				}
			}
			driver.mu.Lock()
			defer driver.mu.Unlock()
			if len(driver.starts) != 1 || driver.pid != 0 {
				t.Fatal("recovery bypassed new-process permission check")
			}
		})
	}
}
