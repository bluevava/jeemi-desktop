package desktop

import (
	"fmt"

	"jeemi/internal/application"
	"jeemi/internal/configtransfer"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type LocalPackageDialogLabels struct {
	Title      string `json:"title"`
	FilterName string `json:"filterName"`
}

type LocalPackageExportResult struct {
	Cancelled bool `json:"cancelled"`
}

func (a *App) ExportLocalPackage(kind, id string, labels LocalPackageDialogLabels) (LocalPackageExportResult, error) {
	ctx := a.context()
	if ctx == nil {
		return LocalPackageExportResult{}, fmt.Errorf("application window is unavailable")
	}
	contents, filename, err := a.service.ExportLocalPackage(kind, id)
	if err != nil {
		return LocalPackageExportResult{}, err
	}
	path, err := wailsRuntime.SaveFileDialog(ctx, wailsRuntime.SaveDialogOptions{
		Title: labels.Title, DefaultFilename: filename,
		Filters: []wailsRuntime.FileFilter{{DisplayName: labels.FilterName, Pattern: "*" + configtransfer.Extension}},
	})
	if err != nil {
		return LocalPackageExportResult{}, fmt.Errorf("cannot open package export dialog")
	}
	if path == "" {
		return LocalPackageExportResult{Cancelled: true}, nil
	}
	return LocalPackageExportResult{}, configtransfer.WriteFile(path, contents)
}

func (a *App) ImportLocalPackage(kind, id string, labels LocalPackageDialogLabels) (application.LocalPackagePreview, error) {
	ctx := a.context()
	if ctx == nil {
		return application.LocalPackagePreview{}, fmt.Errorf("application window is unavailable")
	}
	path, err := wailsRuntime.OpenFileDialog(ctx, wailsRuntime.OpenDialogOptions{
		Title:   labels.Title,
		Filters: []wailsRuntime.FileFilter{{DisplayName: labels.FilterName, Pattern: "*" + configtransfer.Extension}},
	})
	if err != nil {
		return application.LocalPackagePreview{}, fmt.Errorf("cannot open package import dialog")
	}
	if path == "" {
		return application.LocalPackagePreview{Cancelled: true}, nil
	}
	contents, err := configtransfer.ReadFile(path)
	if err != nil {
		return application.LocalPackagePreview{}, err
	}
	return a.service.PreviewLocalPackage(kind, id, contents)
}

func (a *App) ResolveLocalPackage(token string, confirm bool) error {
	return a.service.ResolveLocalPackage(token, confirm)
}
