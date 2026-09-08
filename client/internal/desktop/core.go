package desktop

import (
	"fmt"
	"runtime"

	"jeemi/internal/core"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetMihomoVersionState() (core.VersionManagerState, error) {
	return a.service.MihomoVersionState()
}

func (a *App) CheckMihomoUpdates() (core.VersionManagerState, error) {
	return a.service.CheckMihomoUpdates()
}

func (a *App) DownloadMihomoVersion(version string) (core.VersionManagerState, error) {
	return a.service.DownloadMihomoVersion(version)
}

func (a *App) ImportMihomoCore(labels core.DialogLabels) (core.DialogImportResult, error) {
	ctx := a.context()
	if ctx == nil {
		return core.DialogImportResult{}, fmt.Errorf("application window is unavailable")
	}
	pattern := "*.zip;*.gz;*.exe;*.bin"
	if runtime.GOOS != "windows" {
		// Unix executable files normally have no suffix; validation still checks
		// the bounded file contents and the current platform's executable header.
		pattern = "*"
	}
	path, err := wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title: labels.Title,
		Filters: []wailsRuntime.FileFilter{{
			DisplayName: labels.FilterName,
			Pattern:     pattern,
		}},
	})
	if err != nil {
		return core.DialogImportResult{}, fmt.Errorf("open mihomo core file dialog: %w", err)
	}
	if path == "" {
		state, stateErr := a.service.MihomoVersionState()
		return core.DialogImportResult{Cancelled: true, State: state}, stateErr
	}
	version, state, err := a.service.ImportMihomoCore(path)
	return core.DialogImportResult{ImportedVersion: version, State: state}, err
}

func (a *App) OpenMihomoReleasesPage() error {
	ctx := a.context()
	if ctx == nil {
		return fmt.Errorf("application window is unavailable")
	}
	wailsRuntime.BrowserOpenURL(ctx, core.OfficialReleasesURL)
	return nil
}

func (a *App) CancelMihomoDownload() {
	a.service.CancelMihomoDownload()
}

func (a *App) SelectMihomoVersion(version string) (core.VersionManagerState, error) {
	return a.service.SelectMihomoVersion(version)
}

func (a *App) RemoveMihomoVersion(version string) (core.VersionManagerState, error) {
	return a.service.RemoveMihomoVersion(version)
}

func (a *App) OpenMihomoDirectory() error {
	return a.service.OpenMihomoDirectory()
}
