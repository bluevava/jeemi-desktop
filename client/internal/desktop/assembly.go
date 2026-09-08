//go:build !bindings

package desktop

import (
	"fmt"
	"os"
	"runtime"

	"jeemi/internal/platform/desktop"
	"jeemi/internal/platform/paths"
	apptray "jeemi/internal/platform/tray"
)

// assembleApplication performs runtime-only data preparation. Wails executes a
// special bindings build of main during code generation, so this implementation
// is excluded from that build to keep compilation free of user-data changes.
func assembleApplication(resources Resources) (*App, string, error) {
	dataDirectory, err := paths.DataDirectory()
	if err != nil {
		return nil, "", err
	}
	webViewDirectory, err := paths.WebViewDataDirectoryFromRoot(dataDirectory)
	if err != nil {
		return nil, "", err
	}
	if runtime.GOOS == "windows" {
		migration, err := paths.MigrateLegacyWebViewData(dataDirectory)
		if err != nil {
			return nil, "", err
		}
		if migration.Conflict {
			println("Jeemi startup warning: legacy WebView data was left unchanged because the managed directory already exists:", migration.LegacyDirectory)
		}
		if err := os.MkdirAll(webViewDirectory, 0o700); err != nil {
			return nil, "", fmt.Errorf("create Jeemi data directory: %w", err)
		}
	}
	app, err := NewApp(dataDirectory)
	if err != nil {
		return nil, "", err
	}
	if err := desktop.Prepare(resources.IconPNG); err != nil {
		// Desktop integration failure must not prevent opening or closing Jeemi.
		println("Jeemi desktop registration warning:", err.Error())
	}
	applicationTray, err := apptray.New(resources.trayIcon(), resources.TrayLabels)
	if err != nil {
		return nil, "", err
	}
	app.tray = applicationTray
	return app, webViewDirectory, nil
}
