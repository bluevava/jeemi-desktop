package desktop

import (
	"jeemi/internal/application"
	"jeemi/internal/platform/dialogs"
	"jeemi/internal/platform/tray"
)

func (a *App) trayProxyControls() tray.ProxyControls {
	return tray.ProxyControls{
		State: func() tray.ProxyState {
			service := a.currentService()
			if service == nil || a.quitRequested.Load() {
				return tray.ProxyState{Busy: true}
			}
			state := service.ProxyControlsStatus()
			return tray.ProxyState{Running: state.Running, Busy: state.Busy}
		},
		Start: func() {
			a.runTrayProxyAction((*application.Service).StartProxy, func(labels tray.Labels) string { return labels.StartProxy })
		},
		Stop: func() {
			a.runTrayProxyAction((*application.Service).StopProxy, func(labels tray.Labels) string { return labels.StopProxy })
		},
		Restart: func() {
			a.runTrayProxyAction((*application.Service).RestartProxy, func(labels tray.Labels) string { return labels.RestartProxy })
		},
	}
}

func (a *App) runTrayProxyAction(operation func(*application.Service) (application.RuntimeStatus, error), title func(tray.Labels) string) {
	ctx := a.context()
	service := a.currentService()
	if ctx == nil || service == nil || a.quitRequested.Load() {
		return
	}
	if _, err := operation(service); err != nil && !a.quitRequested.Load() {
		// Do not expose raw errors/paths/credentials in native tray dialogs. The
		// existing home status contains the structured, sanitized failure.
		a.showMainWindow(ctx)
		labels := a.tray.CurrentLabels()
		_ = dialogs.ShowError(ctx, title(labels), labels.ProxyFailure, labels.Close)
	}
}
