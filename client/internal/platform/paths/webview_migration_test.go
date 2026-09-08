package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyWebViewDirectoryMovesOnlyWhenTargetIsAbsent(t *testing.T) {
	configRoot := t.TempDir()
	dataRoot := filepath.Join(configRoot, DataDirectoryName)
	legacy := filepath.Join(configRoot, "Jeemi.exe")
	target := filepath.Join(dataRoot, WebViewDirectoryName)
	if err := os.MkdirAll(filepath.Join(legacy, "EBWebView"), 0o700); err != nil {
		t.Fatalf("create legacy directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "EBWebView", "state"), []byte("preserved"), 0o600); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	result, err := migrateLegacyWebViewDirectory(legacy, target, dataRoot)
	if err != nil {
		t.Fatalf("migrateLegacyWebViewDirectory() error = %v", err)
	}
	if !result.Migrated || result.Conflict {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy directory still exists: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "EBWebView", "state"))
	if err != nil || string(contents) != "preserved" {
		t.Fatalf("migrated state = %q, error = %v", contents, err)
	}
}

func TestMigrateLegacyWebViewDirectoryDoesNotMergeExistingTarget(t *testing.T) {
	configRoot := t.TempDir()
	dataRoot := filepath.Join(configRoot, DataDirectoryName)
	legacy := filepath.Join(configRoot, "Jeemi.exe")
	target := filepath.Join(dataRoot, WebViewDirectoryName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}

	result, err := migrateLegacyWebViewDirectory(legacy, target, dataRoot)
	if err != nil {
		t.Fatalf("migrateLegacyWebViewDirectory() error = %v", err)
	}
	if result.Migrated || !result.Conflict {
		t.Fatalf("unexpected migration result: %+v", result)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory was modified: %v", err)
	}
}
