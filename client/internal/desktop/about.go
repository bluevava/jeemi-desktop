package desktop

import (
	"fmt"

	"jeemi/internal/appupdate"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) OpenJeemiRepository() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, appupdate.RepositoryURL)
	return nil
}

func (a *App) CheckJeemiUpdates() (appupdate.Result, error) {
	return a.service.CheckJeemiUpdates()
}

func (a *App) OpenJeemiCommunity(destination string) error {
	var address string
	switch destination {
	case "group":
		address = "https://t.me/Jeemi_group"
	case "channel":
		address = "https://t.me/Jeemi_channel"
	default:
		return fmt.Errorf("unknown community destination")
	}
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, address)
	return nil
}

func (a *App) GetJeemiUpdateState() appupdate.State { return a.service.JeemiUpdateState() }
func (a *App) CancelJeemiUpdate()                   { a.service.CancelJeemiUpdate() }
func (a *App) DismissJeemiUpdateResult(id string)   { a.service.DismissJeemiUpdateResult(id) }
func (a *App) InstallJeemiUpdate(version string) error {
	ctx := a.context()
	if ctx == nil {
		return appupdate.ErrRestart
	}
	if err := a.service.InstallJeemiUpdate(version); err != nil {
		return err
	}
	go a.requestQuit(ctx)
	return nil
}

func (a *App) OpenJeemiReleases() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, appupdate.ReleasesURL)
	return nil
}
