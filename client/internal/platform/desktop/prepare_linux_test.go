//go:build linux && !bindings

package desktop

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testIcon(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestDesktopRegistrationRefreshesPortablePath(t *testing.T) {
	root := t.TempDir()
	icon := testIcon(t)
	executable := filepath.Join(t.TempDir(), "Jeemi")
	if err := register(root, executable, icon); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(root, "applications", AppID+".desktop")
	entry, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Name=Jeemi\n", "Icon=" + AppID + "\n", "StartupWMClass=" + AppID + "\n", "Exec=" + desktopExec(executable) + "\n"} {
		if !strings.Contains(string(entry), value) {
			t.Fatalf("missing desktop metadata: %s", value)
		}
	}
	before, _ := os.Stat(entryPath)
	if before.Mode().Perm() != 0o644 {
		t.Fatalf("bad desktop permissions: %v", before.Mode())
	}
	if err := register(root, executable, icon); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(entryPath)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged launcher rewritten")
	}
	moved := filepath.Join(t.TempDir(), "中文 portable", "Jeemi")
	if err := register(root, moved, icon); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(entryPath)
	if !strings.Contains(string(updated), "Exec="+desktopExec(moved)+"\n") || bytes.Equal(entry, updated) {
		t.Fatal("portable path was not refreshed")
	}
	installedIcon, err := os.ReadFile(filepath.Join(root, "icons", "hicolor", "256x256", "apps", AppID+".png"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(installedIcon))
	if err != nil || config.Width != 256 || config.Height != 256 {
		t.Fatal("desktop icon not installed")
	}
	if err := filepath.WalkDir(root, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && path != entryPath && !strings.HasSuffix(path, "/"+AppID+".png") {
			t.Fatalf("unexpected registration side effect: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if validator, err := exec.LookPath("desktop-file-validate"); err == nil {
		if output, err := exec.Command(validator, entryPath).CombinedOutput(); err != nil {
			t.Fatalf("invalid desktop entry: %v %s", err, output)
		}
	}
}

func TestDesktopRegistrationRejectsUnsafeOrUnmanagedFiles(t *testing.T) {
	icon := testIcon(t)
	for _, path := range []string{"relative/Jeemi", "/tmp/Jeemi\nHidden=true", "/tmp/Jeemi\x00", "/tmp/equal=name/Jeemi"} {
		if err := register(t.TempDir(), path, icon); err == nil {
			t.Fatalf("invalid path accepted: %q", path)
		}
	}
	for _, symlink := range []bool{false, true} {
		root := t.TempDir()
		entryPath := filepath.Join(root, "applications", AppID+".desktop")
		if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(t.TempDir(), "custom.desktop")
		original := []byte("[Desktop Entry]\nName=Custom\n")
		if err := os.WriteFile(external, original, 0o644); err != nil {
			t.Fatal(err)
		}
		if symlink {
			if err := os.Symlink(external, entryPath); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(entryPath, original, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := register(root, "/tmp/Jeemi", icon); err == nil {
			t.Fatal("unmanaged launcher replaced")
		}
		actual, _ := os.ReadFile(entryPath)
		if !bytes.Equal(actual, original) {
			t.Fatal("existing launcher changed")
		}
		actual, _ = os.ReadFile(external)
		if !bytes.Equal(actual, original) {
			t.Fatal("external file changed")
		}
	}
}

func TestDesktopExecWithGLibLauncher(t *testing.T) {
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("GIO launcher is not installed")
	}
	root := t.TempDir()
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(root, "absent-bus"))
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("GIO_USE_PORTALS", "0")
	outputPath := filepath.Join(root, "launched")
	t.Setenv("JEEMI_DESKTOP_TEST_OUTPUT", outputPath)
	// The launcher executes only this test-owned script, never Jeemi or a GUI.
	directory := filepath.Join(root, "中文 space $() `quote` \"slash\\ percent%f\t")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "Jeemi")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s' \"$0\" > \"$JEEMI_DESKTOP_TEST_OUTPUT\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := register(root, executable, testIcon(t)); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(root, "applications", AppID+".desktop")
	if output, err := exec.Command(gio, "launch", entryPath).CombinedOutput(); err != nil {
		t.Fatalf("GIO launch failed: %v %s", err, output)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(outputPath)
		if err == nil && string(data) == executable {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("desktop Exec quoting did not preserve the literal executable path")
}

func TestXDGDataHome(t *testing.T) {
	for input, expected := range map[string]string{"": "/home/test/.local/share", "relative": "/home/test/.local/share", "/custom/share": "/custom/share"} {
		if actual := dataHome(input, "/home/test"); actual != expected {
			t.Fatalf("%q resolved to %q", input, actual)
		}
	}
}
