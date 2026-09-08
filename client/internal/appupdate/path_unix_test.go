//go:build !windows

package appupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// macOS commonly exposes temporary paths through /var -> /private/var.
// A private directory link also reproduces that path shape on Linux.
func aliasedTempDir(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	directory := filepath.Join(parent, "real")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	return alias
}

func TestMacArchiveLinksThroughDirectoryAlias(t *testing.T) {
	target, _ := targetFor("darwin", "arm64")
	for _, tc := range []struct {
		name, link string
		want       error
	}{
		{"contained", "Contents/MacOS/Jeemi", nil},
		{"missing", "missing", ErrPackage},
		{"escape", "../../outside", ErrPackage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for _, entry := range []struct {
				name, body string
				mode       os.FileMode
			}{
				{target.directory + "/" + target.executable, "gui", 0755},
				{target.directory + "/Jeemi.app/client-link", tc.link, os.ModeSymlink | 0777},
			} {
				header := &zip.FileHeader{Name: entry.name}
				header.SetMode(entry.mode)
				file, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = file.Write([]byte(entry.body)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(t.TempDir(), "package.zip")
			if err := os.WriteFile(archive, buffer.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			output := aliasedTempDir(t)
			if err := extract(context.Background(), archive, output, target); err != tc.want {
				t.Fatalf("extract linked bundle: %v, want %v", err, tc.want)
			}
			if tc.want == nil {
				link := filepath.Join(output, target.directory, "Jeemi.app", "client-link")
				if data, err := os.ReadFile(link); err != nil || string(data) != "gui" {
					t.Fatalf("read contained link: %q, %v", data, err)
				}
			}
		})
	}
}

func TestPackageRejectsFileLinkOutsideAliasedDirectory(t *testing.T) {
	target, _ := targetFor("darwin", "arm64")
	directory := packageFixture(t, aliasedTempDir(t), target, "1.2.3", []byte("gui"))
	if err := verifyPackage(context.Background(), directory, "v1.2.3", target); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("gui"), 0755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, target.executable)
	if err := os.Remove(executable); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, executable); err != nil {
		t.Fatal(err)
	}
	if err := verifyPackage(context.Background(), directory, "v1.2.3", target); err != ErrPackage {
		t.Fatalf("outside file with matching digest accepted: %v", err)
	}
}
