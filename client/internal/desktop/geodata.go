package desktop

import (
	"fmt"

	"jeemi/internal/application"
	"jeemi/internal/geodata"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetGeoDataState() (geodata.State, error) {
	return a.service.GeoDataState()
}

func (a *App) SaveGeoDataPreferences(input geodata.Preferences) (geodata.State, error) {
	return a.service.SaveGeoDataPreferences(input)
}

func (a *App) CheckGeoDataUpdates() (geodata.State, error) {
	return a.service.CheckGeoDataUpdates()
}

func (a *App) DownloadGeoData(kind string) (geodata.State, error) {
	return a.service.DownloadGeoData(kind)
}

func (a *App) DownloadAllGeoData() (geodata.State, error) {
	return a.service.DownloadAllGeoData()
}

func (a *App) CancelGeoDataDownload() {
	a.service.CancelGeoDataDownload()
}

func (a *App) ImportGeoData(kind string, labels geodata.DialogLabels) (geodata.DialogImportResult, error) {
	if _, err := geodata.CanonicalName(geodata.Kind(kind)); err != nil {
		return geodata.DialogImportResult{}, err
	}
	ctx := a.context()
	if ctx == nil {
		return geodata.DialogImportResult{}, fmt.Errorf("application window is unavailable")
	}
	path, err := wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title: labels.Title,
		Filters: []wailsRuntime.FileFilter{{
			DisplayName: labels.FilterName,
			Pattern:     "*.dat;*.mmdb;*.metadb;*.db",
		}},
	})
	if err != nil {
		return geodata.DialogImportResult{}, fmt.Errorf("open GEO data file dialog: %w", err)
	}
	if path == "" {
		state, stateErr := a.service.GeoDataState()
		return geodata.DialogImportResult{Cancelled: true, State: state}, stateErr
	}
	state, err := a.service.ImportGeoData(kind, path)
	return geodata.DialogImportResult{State: state}, err
}

func (a *App) ApplyGeoDataUpdates() (application.RuntimeStatus, error) {
	return a.service.ApplyGeoDataUpdates()
}

func (a *App) CleanOldGeoDataRevisions() (geodata.State, error) {
	return a.service.CleanOldGeoDataRevisions()
}

func (a *App) OpenGeoDataDirectory() error {
	return a.service.OpenGeoDataDirectory()
}

func (a *App) OpenGeoDataReleasesPage() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, geodata.OfficialReleasesURL)
	return nil
}
