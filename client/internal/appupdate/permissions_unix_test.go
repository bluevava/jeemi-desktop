//go:build linux || darwin

package appupdate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestUpdatePermissionsIgnoreUmask(t *testing.T) {
	// umask is process-wide; isolate it from other tests and subprocess workers.
	if os.Getenv("JEEMI_TEST_UPDATE_UMASK") != "1" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(self, "-test.run=^TestUpdatePermissionsIgnoreUmask$")
		command.Env = append(os.Environ(), "JEEMI_TEST_UPDATE_UMASK=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("permission subprocess: %v: %s", err, output)
		}
		return
	}
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	target, _ := targetFor("darwin", "arm64")
	for _, name := range []string{target.executable, target.helper} {
		output := t.TempDir()
		archive := zipFixture(t, target.directory+"/"+name, 0644, []byte("executable"))
		if err := extract(context.Background(), archive, output, target); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(output, target.directory, "Jeemi.app")
		// Copying a read-only bundle must succeed; it is filled before chmod.
		if err := os.Chmod(source, 0555); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "Jeemi.app")
		if err := copyTree(source, destination); err != nil {
			t.Fatal(err)
		}
		for path, mode := range map[string]os.FileMode{
			destination: 0555,
			filepath.Join(destination, "Contents", "MacOS"):                    0755,
			filepath.Join(destination, strings.TrimPrefix(name, "Jeemi.app/")): 0755,
		} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
				t.Fatalf("%s: %v %v, want %v", path, info, err, mode)
			}
		}
		// Restore writable roots so ordinary temp cleanup can remove children.
		_ = os.Chmod(source, 0755)
		_ = os.Chmod(destination, 0755)
	}
}

func TestUnwritableMacInstallParentRejectedWithoutChangingApp(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses ordinary-user directory write permissions")
	}
	target, _ := targetFor("darwin", "arm64")
	root := t.TempDir()
	executable := filepath.Join(root, target.executable)
	if err := os.MkdirAll(filepath.Dir(executable), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("old gui"), 0755); err != nil {
		t.Fatal(err)
	}
	executable = canonicalTestPath(t, executable)
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(root, 0755)
	if err := checkInstallLocation(executable, target, strings.Repeat("e", 32)); err != ErrLocation {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(executable); err != nil || string(body) != "old gui" {
		t.Fatalf("old application changed: %q %v", body, err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatal("failed probe left installation files behind", entries)
	}
}
