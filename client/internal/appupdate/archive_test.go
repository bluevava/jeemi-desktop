package appupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func packageFixture(t *testing.T, parent string, target target, version string, executable []byte) string {
	t.Helper()
	directory := filepath.Join(parent, target.directory)
	manifest := packageManifest{Product: "Jeemi", Version: version, Platform: target.platform, Architecture: target.architecture}
	for name, contents := range map[string][]byte{target.executable: executable, target.helper: []byte("new helper")} {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, contents, 0755); err != nil {
			t.Fatal(err)
		}
		manifest.Files = append(manifest.Files, struct {
			Path   string `json:"path"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		}{name, int64(len(contents)), fmt.Sprintf("%x", sha256.Sum256(contents))})
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(directory, "release.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte("checksums"), 0644); err != nil {
		t.Fatal(err)
	}
	return directory
}

func zipFixture(t *testing.T, name string, mode os.FileMode, body []byte) string {
	t.Helper()
	var output bytes.Buffer
	w := zip.NewWriter(&output)
	h := &zip.FileHeader{Name: name, Method: zip.Deflate}
	h.SetMode(mode)
	file, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(body); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "package.zip")
	if err = os.WriteFile(archive, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return archive
}

func TestArchiveRejectsTraversalAndLinks(t *testing.T) {
	target, _ := targetFor("windows", "amd64")
	for _, name := range []string{"../outside", "/absolute", "Windows-amd64/../../outside", "Windows-amd64/file:stream", "Windows-amd64\\evil", "Windows-amd64/NUL.txt", "Windows-amd64/file.", "wrong/Jeemi.exe"} {
		archive := zipFixture(t, name, 0644, []byte("untrusted"))
		if err := extract(context.Background(), archive, t.TempDir(), target); err != ErrPackage {
			t.Fatalf("%s: %v", name, err)
		}
	}
	archive := zipFixture(t, "Windows-amd64/link", os.ModeSymlink|0777, []byte("outside"))
	if err := extract(context.Background(), archive, t.TempDir(), target); err != ErrPackage {
		t.Fatal(err)
	}
	target, _ = targetFor("darwin", "amd64")
	archive = zipFixture(t, "macOS-amd64/Jeemi.app/link", os.ModeSymlink|0777, []byte("../../outside"))
	if err := extract(context.Background(), archive, t.TempDir(), target); err != ErrPackage {
		t.Fatal(err)
	}
}

func TestTarExtractionAndCancellation(t *testing.T) {
	target, _ := targetFor("linux", "amd64")
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "Linux-amd64/Jeemi", Mode: 0755, Size: 3, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte("app"))
	_ = tw.Close()
	_ = gz.Close()
	archive := filepath.Join(t.TempDir(), "package.tar.gz")
	_ = os.WriteFile(archive, buffer.Bytes(), 0600)
	out := t.TempDir()
	if err := extract(context.Background(), archive, out, target); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(out, target.directory, "Jeemi"))
	if string(data) != "app" {
		t.Fatal(string(data))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := extract(ctx, archive, t.TempDir(), target); err != ErrCancelled {
		t.Fatal(err)
	}
}

func TestPackageManifestRejectsMismatchAndUnlistedFiles(t *testing.T) {
	target, _ := targetFor("windows", "arm64")
	directory := packageFixture(t, t.TempDir(), target, "1.2.3", []byte("gui"))
	if err := verifyPackage(context.Background(), directory, "v1.2.3", target); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(context.Background(), directory, "v1.2.4", target); err != ErrPackage {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(directory, "extra.exe"), []byte("extra"), 0755)
	if err := verifyPackage(context.Background(), directory, "v1.2.3", target); err != ErrPackage {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(directory, "extra.exe"))
	_ = os.WriteFile(filepath.Join(directory, target.helper), []byte("changed"), 0755)
	if err := verifyPackage(context.Background(), directory, "v1.2.3", target); err != ErrPackage {
		t.Fatal(err)
	}
}

func TestMacBundleLayoutAndContainedSymlink(t *testing.T) {
	target, _ := targetFor("darwin", "arm64")
	root := t.TempDir()
	exe := filepath.Join(root, filepath.FromSlash(target.executable))
	if actual, err := target.installRoot(exe); err != nil || actual != root {
		t.Fatalf("%q %v", actual, err)
	}
	if runtime.GOOS == "windows" {
		return
	} // Windows symlink creation may require real elevation.
	directory := packageFixture(t, t.TempDir(), target, "1.2.3", []byte("gui"))
	link := filepath.Join(directory, "Jeemi.app", "client-link")
	if err := os.Symlink("Contents/MacOS/Jeemi", link); err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(t.TempDir(), "Jeemi.app")
	if err := copyTree(filepath.Join(directory, "Jeemi.app"), copy); err != nil {
		t.Fatal(err)
	}
	if value, err := os.Readlink(filepath.Join(copy, "client-link")); err != nil || value != "Contents/MacOS/Jeemi" {
		t.Fatalf("%q %v", value, err)
	}
}

func TestArchiveRejectsCaseCollisions(t *testing.T) {
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	for _, name := range []string{"Windows-amd64/Jeemi.exe", "Windows-amd64/jeemi.exe"} {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte("gui"))
	}
	_ = w.Close()
	archive := filepath.Join(t.TempDir(), "duplicate.zip")
	_ = os.WriteFile(archive, buffer.Bytes(), 0600)
	target, _ := targetFor("windows", "amd64")
	if err := extract(context.Background(), archive, t.TempDir(), target); err != ErrPackage {
		t.Fatal(err)
	}
	if safeArchivePath(strings.Repeat("../", 5) + "outside") {
		t.Fatal("traversal allowed")
	}
}
