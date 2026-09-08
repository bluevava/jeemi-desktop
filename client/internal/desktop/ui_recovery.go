package desktop

import (
	"errors"
	"jeemi/internal/platform/dialogs"
	"jeemi/internal/uidiagnostics"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ReportUIFailure(input uidiagnostics.Failure) error {
	return a.service.ReportUIFailure(input)
}

func (a *App) OpenUIDiagnosticsDirectory() error {
	return a.service.OpenUIDiagnosticsDirectory()
}

// ReloadInterface is also dispatched by the native tray, so a React failure
// cannot remove the recovery entry point. No application/runtime restart occurs.
func (a *App) ReloadInterface() error {
	ctx := a.context()
	if ctx == nil || a.tray == nil || a.quitRequested.Load() {
		return errors.New("interface recovery is unavailable")
	}
	if !a.uiReloadRequested.CompareAndSwap(false, true) {
		return nil
	}
	defer a.uiReloadRequested.Store(false)
	labels := a.tray.CurrentLabels()
	a.showMainWindow(ctx)
	confirmed, err := dialogs.Confirm(ctx, dialogs.Confirmation{
		Title: labels.Reload, Message: labels.ReloadMessage,
		Confirm: labels.ReloadConfirm, Cancel: labels.ReloadCancel,
	})
	if err != nil {
		return errors.New("could not show interface recovery dialog")
	}
	if !confirmed || a.quitRequested.Load() {
		return nil
	}
	// Diagnostics are best effort; slow storage must not delay UI recovery.
	go func() {
		_ = a.service.ReportUIFailure(uidiagnostics.Failure{
			Kind: "manual_reload", Scope: "app", Page: "unknown", ErrorType: "unknown", Code: "unknown",
		})
	}()
	// Wails 2 reloads the original application URL through the WebView. This
	// recovers a cleared React root/error document, but cannot guarantee recovery
	// from a hung browser process or a blocked JavaScript thread.
	wailsRuntime.WindowReloadApp(ctx)
	return nil
}
