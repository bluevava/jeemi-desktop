package desktop

import (
	"context"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"jeemi/internal/platform/sessionend"
)

func (a *App) startSessionWatch(ctx context.Context) {
	stop, err := sessionend.Start(func() { a.quitRequested.Store(true) }, a.cleanupForExit, func() { wailsRuntime.Quit(ctx) })
	if err != nil {
		println("Jeemi session notification unavailable:", err.Error())
		return
	}
	a.stopSessionWatch = stop
}

// OS termination bypasses close-to-tray. Restore networking before removing
// the tray; ordinary Wails OnShutdown reuses this same idempotent cleanup.
func (a *App) cleanupForExit(ctx context.Context) {
	a.quitRequested.Store(true)
	a.screenImport.cancelActive()
	a.exitCleanupOnce.Do(func() {
		a.exitCleanupDone = make(chan struct{})
		go func() {
			defer close(a.exitCleanupDone)
			if a.service != nil {
				_ = a.service.Shutdown(ctx)
			}
			if a.tray != nil {
				a.tray.Stop()
			}
		}()
	})
	select {
	case <-a.exitCleanupDone:
	case <-ctx.Done():
	}
}
