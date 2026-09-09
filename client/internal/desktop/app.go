package desktop

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"jeemi/internal/application"
	"jeemi/internal/platform/paths"
	apptray "jeemi/internal/platform/tray"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const windowActivityEvent = "jeemi:window-activity"

// App is the narrow Wails binding surface exposed to the React renderer.
// System and runtime behavior stays behind the application service.
type App struct {
	service         *application.Service
	tray            *apptray.Controller
	serviceMu       sync.Mutex
	dataDirectory   string
	startupLabels   []byte
	hostReady       chan struct{}
	initializeOnce  sync.Once
	initializeError error

	runtimeMu         sync.RWMutex
	runtimeCtx        context.Context
	quitRequested     atomic.Bool
	uiReloadRequested atomic.Bool
	screenImport      screenImportOperation
	exitCleanupOnce   sync.Once
	exitCleanupDone   chan struct{}
	stopSessionWatch  func()
}

func NewApp(dataDirectory string) (*App, error) {
	if _, err := paths.SettingsFileFromRoot(dataDirectory); err != nil {
		return nil, err
	}
	return &App{dataDirectory: dataDirectory, hostReady: make(chan struct{})}, nil
}

func (a *App) startup(ctx context.Context) {
	a.runtimeMu.Lock()
	a.runtimeCtx = ctx
	a.runtimeMu.Unlock()
	a.startSessionWatch(ctx)
	close(a.hostReady)
}

func (a *App) startTray(ctx context.Context) {
	if a.tray != nil {
		a.tray.Start(
			func() {
				a.showMainWindow(ctx)
			},
			func() {
				// A native dialog must not block the tray's message loop.
				go func() { _ = a.ReloadInterface() }()
			},
			func() {
				// The menu callback runs on the native tray loop. Stop from a
				// separate goroutine so that loop can process its removal message.
				go a.requestQuit(ctx)
			},
			a.trayProxyControls(),
		)
	}
}

func (a *App) shutdown(_ context.Context) {
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a.cleanupForExit(shutdownContext)
	if a.stopSessionWatch != nil {
		a.stopSessionWatch()
	}
	a.runtimeMu.Lock()
	a.runtimeCtx = nil
	a.runtimeMu.Unlock()
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.quitRequested.Load() {
		return false
	}
	if a.tray != nil && a.tray.Ready() {
		a.hideMainWindowToTray(ctx)
		return true
	}
	return false
}

func (a *App) HideWindowToTray() {
	ctx := a.context()
	if ctx == nil {
		return
	}
	if a.tray != nil && a.tray.Ready() {
		a.hideMainWindowToTray(ctx)
		return
	}
	a.requestQuit(ctx)
}

func (a *App) hideMainWindowToTray(ctx context.Context) {
	// Keep the existing WebView alive so the user's route and drafts remain
	// intact, but let the renderer stop live streams, polling and animation
	// before the native window disappears.
	wailsRuntime.EventsEmit(ctx, windowActivityEvent, "hidden")
	wailsRuntime.WindowHide(ctx)
}

func (a *App) showMainWindow(ctx context.Context) {
	// Wake the existing renderer before showing it so it can refresh runtime
	// state without constructing a second window or a second WebView.
	wailsRuntime.EventsEmit(ctx, windowActivityEvent, "visible")
	wailsRuntime.WindowUnminimise(ctx)
	wailsRuntime.WindowShow(ctx)
}

func (a *App) context() context.Context {
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return a.runtimeCtx
}

func (a *App) requestQuit(ctx context.Context) {
	if !a.quitRequested.CompareAndSwap(false, true) {
		return
	}
	a.screenImport.cancelActive()
	if a.tray != nil {
		a.tray.Stop()
	}
	wailsRuntime.Quit(ctx)
}

func (a *App) SetTrayLanguage(language string) {
	if a.tray != nil {
		a.tray.SetLanguage(language)
	}
}
