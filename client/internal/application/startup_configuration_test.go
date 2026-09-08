package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/configfile"
)

func TestStartupCheckDoesNotCreateData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-data")
	if err := CheckStartupConfiguration(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("check created data", err)
	}
}

func TestStartupRecoveryUsesActualReadersAndKeepsUnrelatedData(t *testing.T) {
	root := t.TempDir()
	paths := []string{"settings.json", "local-configs/resources/library.json", "geodata/manifest.json"}
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"invalid":`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(root, "notes.json")
	if err := os.WriteFile(keep, []byte("keep this file"), 0o600); err != nil {
		t.Fatal(err)
	}
	count := 0
	err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
		if count >= len(paths) || f.Path != filepath.Join(root, filepath.FromSlash(paths[count])) || f.Reason != "invalid" {
			t.Fatalf("unexpected failure: %+v", f)
		}
		count++
		return true, nil
	})
	if err != nil || count != len(paths) {
		t.Fatal(err, count)
	}
	if _, err := NewService(root); err != nil {
		t.Fatal("startup still failed after recovery", err)
	}
	contents, err := os.ReadFile(keep)
	if err != nil || string(contents) != "keep this file" {
		t.Fatal("unrelated data changed", err)
	}
}

func TestStartupCheckIdentifiesSemanticErrorsWithoutWriting(t *testing.T) {
	for _, test := range []struct{ path, contents string }{
		{"local-configs/resources/library.json", `{"version":99,"revision":1}`},
		{"local-configs/resources/library.json", `{"version":1,"revision":1,"updatedAt":"invalid"}`},
		{"geodata/manifest.json", `{"version":99}`},
		{"geodata/manifest.json", `{"version":1,"desired":{"geoip":"invalid"}}`},
		{"settings.json", `{"runtime":{"listenPort":"invalid"}}`},
	} {
		t.Run(test.path+test.contents, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			err := CheckStartupConfiguration(root)
			var failure *configfile.Failure
			if !errors.As(err, &failure) || failure.Path != path || failure.Reason != "invalid" {
				t.Fatalf("missing file diagnostic: %v", err)
			}
			contents, _ := os.ReadFile(path)
			if string(contents) != test.contents {
				t.Fatal("read modified the file")
			}
		})
	}
}
