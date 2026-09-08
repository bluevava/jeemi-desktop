//go:build windows

package winauth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func installedCoreFixture(t *testing.T) ([]byte, Target) {
	t.Helper()
	data, target := coreFixture()
	target.ExecutablePath = filepath.Join(t.TempDir(), "带 空格", "jeemi_data", "core", "mihomo", target.Version, "windows-"+runtime.GOARCH, "mihomo.exe")
	if err := os.MkdirAll(filepath.Dir(target.ExecutablePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]any{"version": target.Version, "binarySha256": target.SHA256, "binarySize": target.Size})
	if err := os.WriteFile(filepath.Join(filepath.Dir(target.ExecutablePath), "metadata.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
	return data, target
}

func TestSelectedCoreRejectsFileAndParentReparsePoints(t *testing.T) {
	for _, parent := range []bool{false, true} {
		name := "file"
		if parent {
			name = "parent"
		}
		t.Run(name, func(t *testing.T) {
			_, target := installedCoreFixture(t)
			path := target.ExecutablePath
			if parent {
				path = filepath.Dir(path)
			}
			if err := os.Rename(path, path+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(path+".original", path); err != nil {
				if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
					t.Skip("test environment cannot create symbolic links")
				}
				t.Fatal(err)
			}
			if file, err := OpenInstalledCore(context.Background(), target); err == nil {
				file.Close()
				t.Fatal("reparse point accepted as a selected core")
			}
			if err := os.Remove(path); err != nil {
				t.Fatalf("rejected reparse point remained locked: %v", err)
			}
		})
	}
}

func TestSelectedCoreAndParentPathAreLockedUntilReleased(t *testing.T) {
	data, target := installedCoreFixture(t)
	selected, err := TargetForExecutable(target.ExecutablePath)
	if err != nil || selected != target {
		t.Fatalf("selected target = %+v, %v", selected, err)
	}
	file, err := OpenInstalledCore(context.Background(), selected)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if file.Name() != target.ExecutablePath {
		t.Fatal("opened a different core path")
	}
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err == nil {
		t.Fatal("verified core could be overwritten")
	}
	if err := os.Rename(target.ExecutablePath, target.ExecutablePath+".old"); err == nil {
		t.Fatal("verified core could be replaced")
	}
	for directory := filepath.Dir(target.ExecutablePath); filepath.Base(directory) != "jeemi_data"; directory = filepath.Dir(directory) {
		if err := os.Rename(directory, directory+".old"); err == nil {
			os.Rename(directory+".old", directory)
			t.Fatal("verified core path could be redirected")
		}
	}
	file.Close()
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatalf("core lock was not released: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(target.ExecutablePath))
	if err != nil || len(entries) != 2 {
		t.Fatal("verification created another file")
	}
}

func TestInstalledCoreRechecksHashAndRejectsUnmanagedTargets(t *testing.T) {
	data, target := installedCoreFixture(t)
	data[len(data)-1]++
	if err := os.WriteFile(target.ExecutablePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if file, err := OpenInstalledCore(context.Background(), target); err == nil {
		file.Close()
		t.Fatal("changed installed core accepted")
	} else if err.Error() != "authorization_core_changed" {
		t.Fatal(err)
	}
	for _, name := range []string{"relative/mihomo.exe", filepath.Join(t.TempDir(), "mihomo.exe"), target.ExecutablePath + ":stream", filepath.Join(filepath.Dir(target.ExecutablePath), "other.exe"), `\\server\share\jeemi_data\core\mihomo\` + target.Version + `\windows-` + runtime.GOARCH + `\mihomo.exe`} {
		candidate := target
		candidate.ExecutablePath = name
		if managedCorePath(candidate) {
			t.Fatalf("unmanaged executable accepted: %s", name)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := OpenInstalledCore(ctx, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled check continued: %v", err)
	}
	if err := os.Rename(filepath.Dir(target.ExecutablePath), filepath.Dir(target.ExecutablePath)+".released"); err != nil {
		t.Fatalf("failed verification retained directory locks: %v", err)
	}
}
