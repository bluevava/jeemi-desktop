package application

import (
	"testing"

	mihomoruntime "jeemi/internal/mihomo/runtime"
)

func TestProxyControlStatusDisablesOperationsDuringTransitions(t *testing.T) {
	for _, state := range []mihomoruntime.State{
		mihomoruntime.StateStopped, mihomoruntime.StateFailed, mihomoruntime.StateRunning,
		mihomoruntime.StatePreparing, mihomoruntime.StateValidating, mihomoruntime.StateStarting,
		mihomoruntime.StateReloading, mihomoruntime.StateStopping, mihomoruntime.StateRecovering,
	} {
		t.Run(string(state), func(t *testing.T) {
			want := ProxyControlStatus{Running: state == mihomoruntime.StateRunning,
				Busy: state != mihomoruntime.StateRunning && state != mihomoruntime.StateStopped && state != mihomoruntime.StateFailed}
			if got := proxyControlStatus(state, RuntimeConfigurationApplied, false); got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
			if got := proxyControlStatus(state, RuntimeConfigurationApplied, true); !got.Busy {
				t.Fatal("pending operation allowed tray actions")
			}
		})
	}
	for _, configuration := range []RuntimeConfigurationState{RuntimeConfigurationBuilding, RuntimeConfigurationValidating, RuntimeConfigurationApplying} {
		if got := proxyControlStatus(mihomoruntime.StateRunning, configuration, false); !got.Busy {
			t.Fatalf("configuration %s allowed concurrent tray action", configuration)
		}
	}
	if got := proxyControlStatus(mihomoruntime.StateRunning, RuntimeConfigurationFailed, false); got.Busy || !got.Running {
		t.Fatal("rejected configuration hid the healthy running core")
	}
}

func TestProxyControlsStatusNeedsNoSettingsOrPlatformIO(t *testing.T) {
	manager, err := mihomoruntime.NewManager(mihomoruntime.Options{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately omit settings, GEO, version and authorization services.
	service := &Service{runtimeManager: manager}
	if got := service.ProxyControlsStatus(); got != (ProxyControlStatus{}) {
		t.Fatalf("unexpected idle controls: %+v", got)
	}
	service.coreOperationCancel = func() {}
	if !service.ProxyControlsStatus().Busy {
		t.Fatal("authorization not reflected")
	}
	service.coreOperationCancel = nil
	service.shuttingDown = true
	if !service.ProxyControlsStatus().Busy {
		t.Fatal("shutdown not reflected")
	}
}
