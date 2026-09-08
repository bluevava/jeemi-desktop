package configfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func brokenFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("broken configuration"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func checkFiles(paths ...string) func() error {
	return func() error {
		for _, path := range paths {
			contents, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return Unreadable(path, err)
			}
			return Invalid(path, contents, errors.New("invalid document"))
		}
		return nil
	}
}

func TestRecoveryDeletesOnlyEachConfirmedFile(t *testing.T) {
	root := t.TempDir()
	first, second := brokenFile(t, root, "first.json"), brokenFile(t, root, "second.json")
	untouched := brokenFile(t, root, "unrelated.json")
	var confirmed []string
	err := Recover(context.Background(), root, checkFiles(first, second), func(f *Failure) (bool, error) {
		if _, err := os.Stat(f.Path); err != nil {
			t.Fatal("deleted before confirmation", err)
		}
		confirmed = append(confirmed, f.Path)
		return true, nil
	})
	if err != nil || len(confirmed) != 2 || confirmed[0] != first || confirmed[1] != second {
		t.Fatalf("recovery: %v, %v", err, confirmed)
	}
	if _, err := os.Stat(untouched); err != nil {
		t.Fatal("unrelated file was affected", err)
	}
	if err := checkFiles(first, second)(); err != nil {
		t.Fatal("confirmed files remain", err)
	}
}

func TestDecliningKeepsCurrentAndLaterFiles(t *testing.T) {
	root := t.TempDir()
	first, second := brokenFile(t, root, "first.json"), brokenFile(t, root, "second.json")
	calls := 0
	err := Recover(context.Background(), root, checkFiles(first, second), func(*Failure) (bool, error) { calls++; return false, nil })
	if !errors.Is(err, ErrDeclined) || calls != 1 {
		t.Fatal(err, calls)
	}
	for _, path := range []string{first, second} {
		contents, err := os.ReadFile(path)
		if err != nil || string(contents) != "broken configuration" {
			t.Fatal("decline changed data", err)
		}
	}
}

func TestRecoveryRechecksChangedFileAndCancellation(t *testing.T) {
	for _, mode := range []string{"changed", "cancelled", "directory", "dialog_error"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := brokenFile(t, root, "settings.json")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := Recover(ctx, root, checkFiles(path), func(*Failure) (bool, error) {
				switch mode {
				case "changed":
					if err := os.WriteFile(path, []byte("repaired"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "cancelled":
					cancel()
				case "directory":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				case "dialog_error":
					return false, errors.New("dialog unavailable")
				}
				return true, nil
			})
			if err == nil {
				t.Fatal("unsafe recovery succeeded")
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal("file was deleted", err)
			}
			if mode == "changed" {
				contents, _ := os.ReadFile(path)
				if string(contents) != "repaired" {
					t.Fatal("changed file was not retained")
				}
			}
		})
	}
}

func TestReadErrorsAndOutsidePathsNeverOfferDeletion(t *testing.T) {
	root := t.TempDir()
	outside := brokenFile(t, t.TempDir(), "outside.json")
	for _, err := range []error{Unreadable(filepath.Join(root, "settings.json"), os.ErrPermission), checkFiles(outside)()} {
		result := Recover(context.Background(), root, func() error { return err }, func(*Failure) (bool, error) { t.Fatal("deletion offered"); return true, nil })
		if result == nil {
			t.Fatal("invalid path succeeded")
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}

func TestLinkedConfigurationCannotBeDeleted(t *testing.T) {
	root := t.TempDir()
	outside := brokenFile(t, t.TempDir(), "outside.json")
	link := filepath.Join(root, "settings.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink unavailable on this test host")
	}
	err := Recover(context.Background(), root, checkFiles(link), func(*Failure) (bool, error) { t.Fatal("linked file offered for deletion"); return true, nil })
	if err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}
