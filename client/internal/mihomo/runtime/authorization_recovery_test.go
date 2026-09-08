package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

// A service can exit before the GUI's periodic watcher closes Done.
type authorizationTestProcess struct {
	*fakeProcess
	alive atomic.Bool
}

func (p *authorizationTestProcess) IsRunning(context.Context) (bool, error) {
	return p.alive.Load(), nil
}

type authorizationTestDriver struct {
	fakeRuntimeDriver
	process *authorizationTestProcess
}

func (d *authorizationTestDriver) Start(executable, home, configuration string) (Process, error) {
	p, err := d.fakeRuntimeDriver.Start(executable, home, configuration)
	if err != nil {
		return nil, err
	}
	d.process = &authorizationTestProcess{fakeProcess: p.(*fakeProcess)}
	d.process.alive.Store(true)
	return d.process, nil
}

func authorizationTestManager(t *testing.T, driver Driver, proxy SystemProxyController) *Manager {
	t.Helper()
	m, err := NewManager(Options{
		DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: proxy,
		HTTPClient: healthyHTTPClient(), GOOS: "windows", RestartDelays: []time.Duration{time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return m
}

func waitAuthorizationState(t *testing.T, m *Manager, predicate func(Status) bool) Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if status := m.Status(); predicate(status) {
			// Wait for the exit/recovery operation to finish its file cleanup.
			m.operationMu.Lock()
			m.operationMu.Unlock()
			return m.Status()
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime did not reach expected authorization state: %+v", m.Status())
	return Status{}
}

func TestHelperPromptKeepsHealthyCoreAndRestoresUnexpectedExitRecovery(t *testing.T) {
	driver := &authorizationTestDriver{}
	m := authorizationTestManager(t, driver, &fakeSystemProxy{})
	if _, err := m.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err != nil {
		t.Fatal(err)
	}
	p := driver.process
	release := m.HoldHelperRecovery()
	if status := m.Status(); status.State != StateRunning || !status.DesiredRunning || processExited(p) {
		t.Fatalf("native prompt interrupted healthy core: %+v", status)
	}
	release(true)
	p.crash(context.Canceled)
	waitAuthorizationState(t, m, func(s Status) bool { return s.State == StateRunning && s.PID != p.PID() })
}

func TestHelperReplacementDoesNotRecoverOldGenerationBeforeWatcherPoll(t *testing.T) {
	driver := &authorizationTestDriver{}
	m := authorizationTestManager(t, driver, &fakeSystemProxy{})
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	p := driver.process
	configuration := m.active.ConfigPath
	release := m.HoldHelperRecovery()
	p.alive.Store(false) // Installer stopped the service; Done has not arrived.
	release(true)
	status := m.Status()
	if status.State != StateStopped || status.DesiredRunning || status.ControllerReady {
		t.Fatalf("replacement retained or revived old generation: %+v", status)
	}
	if _, err := os.Stat(configuration); !os.IsNotExist(err) {
		t.Fatalf("replacement retained old control secret: %v", err)
	}
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatalf("explicit start after replacement: %v", err)
	}
	p.crash(context.Canceled) // A late watcher must not detach the new process.
	if status = m.Status(); status.State != StateRunning || status.PID == p.PID() {
		t.Fatalf("late old watcher affected explicit new start: %+v", status)
	}
}

func TestCancelledHelperSetupDoesNotRecoverAfterLateInstallerExit(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	m := authorizationTestManager(t, driver, &fakeSystemProxy{})
	if _, err := m.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err != nil {
		t.Fatal(err)
	}
	p := driver.starts[0]
	configuration := m.active.ConfigPath
	release := m.HoldHelperRecovery()
	release(false)
	release(true) // A late success must not override a cancellation.
	p.crash(context.Canceled)
	status := waitAuthorizationState(t, m, func(s Status) bool { return s.State == StateStopped })
	if status.DesiredRunning || status.ControllerReady {
		t.Fatalf("cancelled helper setup retained a start intent: %+v", status)
	}
	if _, err := os.Stat(configuration); !os.IsNotExist(err) {
		t.Fatalf("cancelled setup retained old control secret: %v", err)
	}
}

func TestHelperSetupCancelsAlreadyQueuedRecovery(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	m := authorizationTestManager(t, driver, &fakeSystemProxy{})
	m.restartDelays = []time.Duration{time.Hour}
	if _, err := m.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err != nil {
		t.Fatal(err)
	}
	driver.starts[0].crash(context.Canceled)
	waitAuthorizationState(t, m, func(s Status) bool { return s.State == StateRecovering })
	m.HoldHelperRecovery()(false)
	if status := m.Status(); status.DesiredRunning || status.State != StateStopped {
		t.Fatalf("queued recovery survived helper setup: %+v", status)
	}
}

func TestRepairedHelperCanRetryFailedPrimaryStartupRecovery(t *testing.T) {
	proxy := &fakeSystemProxy{recoverErr: context.DeadlineExceeded}
	m := authorizationTestManager(t, &fakeRuntimeDriver{}, proxy)
	if err := m.RecoverSystemProxy(context.Background()); err == nil {
		t.Fatal("missing startup failure")
	}
	if err := m.RetryStartupRecovery(context.Background()); err == nil {
		t.Fatal("failed network recovery was reported as repaired")
	}
	proxy.recoverErr = nil
	if err := m.RetryStartupRecovery(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err != nil {
		t.Fatalf("repaired startup failure still blocks start: %v", err)
	}
	configuration := m.active.ConfigPath
	proxy.recoverErr = context.Canceled
	if err := m.RetryStartupRecovery(context.Background()); err != nil {
		t.Fatalf("retry touched an active session: %v", err)
	}
	if _, err := os.Stat(configuration); err != nil {
		t.Fatalf("retry erased active configuration: %v", err)
	}
}

func TestStartupAuthorizationPrerequisitesWaitForExplicitRecovery(t *testing.T) {
	for _, code := range []string{"authorization_install_required", "authorization_update_required", "authorization_repair_required", "authorization_bundle_required"} {
		t.Run(code, func(t *testing.T) {
			proxy := &fakeSystemProxy{recoverErr: fmt.Errorf("restore: %w", &requirements.Error{Code: code, Message: code})}
			driver := &fakeRuntimeDriver{}
			m := authorizationTestManager(t, driver, proxy)
			assertPending := func() {
				t.Helper()
				status := m.Status()
				if status.State != StateStopped || status.LastError != nil || status.DesiredRunning || status.ControllerReady || len(driver.starts) != 0 {
					t.Fatalf("authorization prerequisite became a runtime failure or start: %+v", status)
				}
			}
			if err := m.RecoverSystemProxy(context.Background()); requirements.Code(err, "") != code {
				t.Fatalf("pending recovery lost its cause: %v", err)
			}
			assertPending()
			request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
			if _, err := m.Start(context.Background(), request); requirements.Code(err, "") != code {
				t.Fatalf("start bypassed pending authorization/recovery: %v", err)
			}
			assertPending()
			if err := m.RetryStartupRecovery(context.Background()); requirements.Code(err, "") != code {
				t.Fatalf("unchanged helper was reported as recovered: %v", err)
			}
			assertPending()
			proxy.recoverErr = nil
			if err := m.RetryStartupRecovery(context.Background()); err != nil {
				t.Fatal(err)
			}
			if status, err := m.Start(context.Background(), request); err != nil || status.State != StateRunning {
				t.Fatalf("authorized recovery still blocks start: %+v %v", status, err)
			}
		})
	}
}

func TestStartupAuthorizationDoesNotHideRecoveryFailures(t *testing.T) {
	for _, scenario := range []string{"network_recovery", "session_cleanup"} {
		t.Run(scenario, func(t *testing.T) {
			code := "authorization_recovery_failed"
			if scenario == "session_cleanup" {
				code = "authorization_update_required"
			}
			driver := &fakeRuntimeDriver{}
			m := authorizationTestManager(t, driver, &fakeSystemProxy{recoverErr: &requirements.Error{Code: code, Message: code}})
			if scenario == "session_cleanup" {
				// A nonempty directory cannot be erased as a stale session file.
				path := filepath.Join(m.store.generations, "stale", "config.yaml")
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.RecoverSystemProxy(context.Background()); err == nil {
				t.Fatal("missing recovery failure")
			}
			if status := m.Status(); status.State != StateFailed || status.LastError == nil || status.LastError.Code != "startup_recovery_failed" {
				t.Fatalf("real recovery failure was hidden: %+v", status)
			}
			if status, err := m.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)); err == nil || status.State != StateFailed || len(driver.starts) != 0 {
				t.Fatalf("failed recovery did not block start: %+v %v", status, err)
			}
		})
	}
}
