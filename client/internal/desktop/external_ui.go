package desktop

import (
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"jeemi/internal/application"
)

func (a *App) GetZashboardState() (application.ZashboardState, error) {
	return a.service.ZashboardState()
}
func (a *App) CheckZashboardUpdates() (application.ZashboardState, error) {
	return a.service.CheckZashboardUpdates()
}
func (a *App) DownloadZashboardVersion(version string) (application.ZashboardState, error) {
	return a.service.DownloadZashboardVersion(version)
}
func (a *App) SelectZashboardVersion(version string) (application.ZashboardState, error) {
	return a.service.SelectZashboardVersion(version)
}
func (a *App) CancelZashboardDownload() { a.service.CancelZashboardDownload() }
func (a *App) OpenZashboard() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	return a.service.OpenZashboard(func(address string) { wailsRuntime.BrowserOpenURL(ctx, address) })
}
