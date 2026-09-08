package application

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/core"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

type startupAuthorizationProxy struct {
	fallbackTestSystemProxy
	ready       atomic.Bool
	recoveries  atomic.Int32
	recoveryErr error
}

func (p *startupAuthorizationProxy) Recover(context.Context) error {
	p.recoveries.Add(1)
	if !p.ready.Load() {
		return &requirements.Error{Code: "authorization_update_required", Message: "authorization_update_required"}
	}
	return p.recoveryErr
}

func TestStartupDefersHelperUpdateUntilExplicitStart(t *testing.T) {
	for _, recoveryErr := range []error{nil, context.DeadlineExceeded} {
		name := "updated_helper"
		if recoveryErr != nil {
			name = "network_recovery_failed"
		}
		t.Run(name, func(t *testing.T) {
			proxy := &startupAuthorizationProxy{recoveryErr: recoveryErr}
			s, driver := authorizationServiceWithProxy(t, runtimeconfig.ProxyModeSystemProxy, proxy)
			var authorizations atomic.Int32
			s.authorizeCore = func(context.Context, coreauth.Target) error {
				authorizations.Add(1)
				proxy.ready.Store(true)
				return nil
			}
			s.Startup(context.Background())
			deadline := time.Now().Add(3 * time.Second)
			for {
				configuration := s.runtimeConfigurationStatus()
				if configuration.Trigger == reconcileTriggerStartup && configuration.State == RuntimeConfigurationReady {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("startup snapshot did not finish: %+v", configuration)
				}
				time.Sleep(time.Millisecond)
			}
			status := s.RuntimeStatus()
			if status.Core.State != core.StateReady || status.Mihomo.State != mihomoruntime.StateStopped || status.Mihomo.LastError != nil || status.Configuration.LastError != nil || status.CoreAuthorizationPending || status.StatusMessageKey != "runtime.status.coreReady" {
				t.Fatalf("idle helper update was displayed as a runtime failure: %+v", status)
			}
			if authorizations.Load() != 0 || driver.starts.Load() != 0 || proxy.recoveries.Load() != 1 {
				t.Fatal("opening the GUI requested authorization or started a core")
			}
			status, err := s.StartProxy()
			if authorizations.Load() != 1 || proxy.recoveries.Load() != 2 {
				t.Fatalf("explicit start did not authorize and retry recovery: %d/%d", authorizations.Load(), proxy.recoveries.Load())
			}
			if recoveryErr != nil {
				if err == nil || status.Mihomo.State != mihomoruntime.StateFailed || status.Mihomo.LastError == nil || status.Mihomo.LastError.Code != "startup_recovery_failed" || driver.starts.Load() != 0 {
					t.Fatalf("real network recovery failure was ignored: %+v %v", status, err)
				}
			} else if err != nil || status.Mihomo.State != mihomoruntime.StateRunning || status.Mihomo.LastError != nil || driver.starts.Load() != 1 {
				t.Fatalf("helper update required restarting the GUI: %+v %v", status, err)
			}
		})
	}
}
