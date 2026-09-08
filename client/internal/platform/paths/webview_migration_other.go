//go:build !windows

package paths

// MigrateLegacyWebViewData is a no-op outside Windows because the legacy
// %APPDATA%/<binary>.exe default belongs to the WebView2 backend.
func MigrateLegacyWebViewData(dataRoot string) (WebViewMigrationResult, error) {
	targetDirectory, err := WebViewDataDirectoryFromRoot(dataRoot)
	if err != nil {
		return WebViewMigrationResult{}, err
	}
	return WebViewMigrationResult{TargetDirectory: targetDirectory}, nil
}
