//go:build windows

package paths

import "path/filepath"

// MigrateLegacyWebViewData moves Wails' old default %APPDATA%/Jeemi.exe
// directory below jeemi_data before WebView2 starts. Existing managed data is
// never merged or overwritten.
func MigrateLegacyWebViewData(dataRoot string) (WebViewMigrationResult, error) {
	targetDirectory, err := WebViewDataDirectoryFromRoot(dataRoot)
	if err != nil {
		return WebViewMigrationResult{}, err
	}
	legacyDirectory := filepath.Join(filepath.Dir(filepath.Clean(dataRoot)), "Jeemi.exe")
	return migrateLegacyWebViewDirectory(legacyDirectory, targetDirectory, dataRoot)
}
