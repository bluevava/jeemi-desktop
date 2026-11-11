package appupdate

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A temporary application, signed on macOS using only a copy of the test
// executable. It never registers a helper or starts a real desktop client.
func macPackageFixture(t *testing.T, target target, version, marker string) string {
	t.Helper()
	directory := packageFixture(t, t.TempDir(), target, version, []byte("test gui"))
	app := filepath.Join(directory, "Jeemi.app")
	info := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.jeemi.update-test</string><key>CFBundleExecutable</key><string>Jeemi</string><key>CFBundlePackageType</key><string>APPL</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(info), 0644); err != nil {
		t.Fatal(err)
	}
	resources := filepath.Join(app, "Contents", "Resources")
	if err := os.Mkdir(resources, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resources, "version.txt"), []byte(marker), 0644); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(self)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{target.executable, target.helper} {
			if err := os.WriteFile(filepath.Join(directory, name), body, 0755); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{filepath.Join(directory, target.helper), app} {
			if output, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--timestamp=none", name).CombinedOutput(); err != nil {
				t.Fatalf("sign test bundle: %v: %s", err, output)
			}
		}
	}
	files := []map[string]any{}
	if err := filepath.WalkDir(app, func(filename string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		digest, size, err := hashFile(filename)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, filename)
		files = append(files, map[string]any{"path": filepath.ToSlash(relative), "sha256": digest, "size": size})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"product": "Jeemi", "version": version, "platform": target.platform, "architecture": target.architecture, "files": files})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "release.json"), body, 0644); err != nil {
		t.Fatal(err)
	}
	return directory
}

func macZipFixture(t *testing.T, directory string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "mac.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	err = filepath.WalkDir(directory, func(filename string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(directory), filename)
		if err != nil {
			return err
		}
		// Deliberately omit Unix executable bits, like a ZIP repacked on Windows.
		header := &zip.FileHeader{Name: filepath.ToSlash(relative), Method: zip.Deflate}
		header.SetMode(0644)
		output, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		_, err = output.Write(body)
		return err
	})
	closeErr := writer.Close()
	fileErr := file.Close()
	if err != nil || closeErr != nil || fileErr != nil {
		t.Fatalf("ZIP fixture: %v / %v / %v", err, closeErr, fileErr)
	}
	return archive
}

func TestMacZipWholeBundleReplacementAndRollback(t *testing.T) {
	target, _ := targetFor("darwin", runtime.GOARCH)
	previous := macPackageFixture(t, target, "1.2.2", "old")
	next := macPackageFixture(t, target, "1.2.3", "new")
	archive := macZipFixture(t, next)
	for _, failure := range []string{"", "permission", "replace", "restart"} {
		t.Run("failure="+failure, func(t *testing.T) {
			root, jobRoot := t.TempDir(), t.TempDir()
			if err := copyTree(filepath.Join(previous, "Jeemi.app"), filepath.Join(root, "Jeemi.app")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			executable := canonicalTestPath(t, filepath.Join(root, target.executable))
			id := strings.Repeat("d", 32)
			if err := checkInstallLocation(executable, target, id); err != nil {
				t.Fatal(err)
			}
			extracted := filepath.Join(jobRoot, "extracted")
			if err := os.Mkdir(extracted, 0700); err != nil {
				t.Fatal(err)
			}
			if err := extract(context.Background(), archive, extracted, target); err != nil {
				t.Fatal(err)
			}
			digest, _, _ := hashFile(executable)
			job := installJob{ID: id, Executable: executable, CurrentSHA256: digest, Version: "1.2.3"}
			r, err := prepareReplacement(context.Background(), jobRoot, job, target)
			if err != nil {
				t.Fatal(err)
			}
			defer r.cleanup()
			for _, name := range []string{target.executable, target.helper} {
				if info, err := os.Stat(filepath.Join(r.stage, "new", name)); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0755) {
					t.Fatalf("staged executable mode: %v %v", info, err)
				}
			}
			if failure == "permission" || failure == "replace" {
				r.rename = func(from, to string) error {
					if failure == "permission" && from == filepath.Join(r.root, "Jeemi.app") {
						return os.ErrPermission
					}
					if failure == "replace" && from == filepath.Join(r.stage, "new", "Jeemi.app") {
						return errors.New("replacement failed")
					}
					return os.Rename(from, to)
				}
			}
			want := map[string]error{"permission": ErrLocation, "replace": ErrReplace, "restart": ErrRestart}[failure]
			if err := completeUpdate(r, func() error {
				if failure == "restart" {
					return ErrRestart
				}
				return nil
			}); err != want {
				t.Fatalf("got %v, want %v", err, want)
			}
			marker := "new"
			if failure != "" {
				marker = "old"
			}
			for name, expected := range map[string]string{"Jeemi.app/Contents/Resources/version.txt": marker, "keep.txt": "untouched"} {
				if body, err := os.ReadFile(filepath.Join(root, name)); err != nil || string(body) != expected {
					t.Fatalf("%s: %q %v", name, body, err)
				}
			}
		})
	}
}
