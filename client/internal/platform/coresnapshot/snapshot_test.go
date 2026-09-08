package coresnapshot

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsFilePathsCannotLeaveProtectedSession(t *testing.T) {
	for _, value := range []string{`C:\Windows\system.ini`, `\\server\share\secret`, `..\secret`, `cache/../../secret`, `/etc/passwd`, `C:/protected/session/../secret`} {
		if safeWindowsPath(value, `C:\protected\session`) == nil {
			t.Fatal(value)
		}
	}
	for _, value := range []string{`C:\protected\session\providers\one.yaml`, `cache/one.yaml`, `https://example.test/resource`, `DIRECT`} {
		if err := safeWindowsPath(value, `C:\protected\session`); err != nil {
			t.Fatal(value, err)
		}
	}
}

func TestSnapshotRejectsArchiveLinksAndTraversal(t *testing.T) {
	for _, header := range []*tar.Header{{Name: "../outside", Size: 1, Typeflag: tar.TypeReg}, {Name: "link", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink}, {Name: "hardlink", Linkname: "/etc/passwd", Typeflag: tar.TypeLink}, {Name: "file", Typeflag: tar.TypeFifo}} {
		var data bytes.Buffer
		writer := tar.NewWriter(&data)
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			writer.Write([]byte("x"))
		}
		writer.Close()
		if Receive(t.TempDir(), "/user/runtime", &data, map[string]string{}) == nil {
			t.Fatal(header.Name)
		}
	}
}
func TestSnapshotCopiesIndependentBytesAndPreservesUpdatedCache(t *testing.T) {
	source := t.TempDir()
	generation := filepath.Join(source, "generations", "one")
	os.MkdirAll(generation, 0700)
	os.WriteFile(filepath.Join(generation, "bootstrap.yaml"), []byte(testController+"mode: rule\n"), 0600)
	os.WriteFile(filepath.Join(generation, "config.yaml"), []byte(testController+"mode: global\n"), 0600)
	os.WriteFile(filepath.Join(source, "cache.db"), []byte("original"), 0600)
	destination := t.TempDir()
	digests := map[string]string{}
	transfer := func() {
		t.Helper()
		var archive bytes.Buffer
		if err := WriteSnapshot(source, "generations/one/bootstrap.yaml", true, &archive); err != nil {
			t.Fatal(err)
		}
		if err := Receive(destination, source, &archive, digests); err != nil {
			t.Fatal(err)
		}
	}
	transfer()
	os.WriteFile(filepath.Join(destination, "cache.db"), []byte("live update"), 0600)
	transfer()
	data, _ := os.ReadFile(filepath.Join(destination, "cache.db"))
	if string(data) != "live update" {
		t.Fatal("unchanged user input replaced live cache")
	}
	os.WriteFile(filepath.Join(source, "cache.db"), []byte("new user input"), 0600)
	transfer()
	data, _ = os.ReadFile(filepath.Join(destination, "cache.db"))
	if string(data) != "new user input" {
		t.Fatal(string(data))
	}
}
func TestProtectedConfigurationPathRewrite(t *testing.T) {
	result, err := rewriteRuntimePaths([]byte(testController+"path: /user/runtime/providers/a.yaml\nmode: rule\n"), "/user/runtime", "/protected/session")
	if err != nil || !strings.Contains(string(result), "/protected/session/providers/a.yaml") || !strings.Contains(string(result), "mode: rule") {
		t.Fatal(string(result), err)
	}
	if _, err := rewriteRuntimePaths([]byte("a: &ref [a]\nb: *ref\n"), "/user/runtime", "/protected/session"); err == nil {
		t.Fatal("alias allowed")
	}
	for _, name := range []string{"../config.yaml", "generations/../config.yaml", "generations/a/../../config.yaml", "generations/a/exec.sh", "/generations/a/config.yaml"} {
		if ConfigurationName(name) {
			t.Fatal(name)
		}
	}
}

const testController = "external-controller: 127.0.0.1:45678\nsecret: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"

func TestProtectedControllerCannotExposeUnauthenticatedAPI(t *testing.T) {
	for _, input := range []string{"mode: rule\n", strings.ReplaceAll(testController, "127.0.0.1", "0.0.0.0"), strings.ReplaceAll(testController, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "short"), testController + "external-controller-unix: /tmp/control.sock\n"} {
		if _, err := rewriteRuntimePaths([]byte(input), "/source", "/protected"); err == nil {
			t.Fatal("unsafe control API accepted")
		}
	}
	if checkBootstrap([]byte(testController+"tun:\n  enable: true\n")) == nil {
		t.Fatal("bootstrap enabled TUN before health check")
	}
}
