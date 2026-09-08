package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WebViewMigrationResult struct {
	LegacyDirectory string
	TargetDirectory string
	Migrated        bool
	Conflict        bool
}

func migrateLegacyWebViewDirectory(legacyDirectory, targetDirectory, dataRoot string) (WebViewMigrationResult, error) {
	result := WebViewMigrationResult{
		LegacyDirectory: filepath.Clean(legacyDirectory),
		TargetDirectory: filepath.Clean(targetDirectory),
	}
	cleanRoot, err := validateDataRoot(dataRoot)
	if err != nil {
		return result, err
	}
	if !filepath.IsAbs(result.LegacyDirectory) || !filepath.IsAbs(result.TargetDirectory) {
		return result, fmt.Errorf("WebView migration paths must be absolute")
	}
	expectedTarget := filepath.Join(cleanRoot, WebViewDirectoryName)
	if result.TargetDirectory != expectedTarget {
		return result, fmt.Errorf("WebView migration target is outside the managed location")
	}
	expectedLegacyParent := filepath.Dir(cleanRoot)
	if filepath.Dir(result.LegacyDirectory) != expectedLegacyParent || !strings.EqualFold(filepath.Base(result.LegacyDirectory), "Jeemi.exe") {
		return result, fmt.Errorf("unexpected legacy WebView directory")
	}

	legacyInfo, err := os.Stat(result.LegacyDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect legacy WebView directory: %w", err)
	}
	if !legacyInfo.IsDir() {
		return result, fmt.Errorf("legacy WebView path is not a directory")
	}
	if _, err := os.Stat(result.TargetDirectory); err == nil {
		result.Conflict = true
		return result, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("inspect managed WebView directory: %w", err)
	}
	if err := os.MkdirAll(cleanRoot, 0o700); err != nil {
		return result, fmt.Errorf("create Jeemi data root for WebView migration: %w", err)
	}
	if err := os.Rename(result.LegacyDirectory, result.TargetDirectory); err != nil {
		return result, fmt.Errorf("move legacy WebView directory into jeemi_data: %w", err)
	}
	result.Migrated = true
	return result, nil
}
