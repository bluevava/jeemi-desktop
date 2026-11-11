package configfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryRepairsOnlyConfirmedContentAndKeepsOriginal(t *testing.T) {
	root := t.TempDir()
	path := brokenFile(t, root, "metadata.json")
	original, _ := os.ReadFile(path)
	replacement := []byte(`{"version":1}`)
	failure := InvalidWithRepair(path, original, replacement, errors.New("unknown field")).(*Failure)
	replacement[0] = 'X' // The prepared candidate must own its contents.
	confirmed := 0
	err := Recover(context.Background(), root, func() error {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(current) == `{"version":1}` {
			return nil
		}
		return failure
	}, func(f *Failure) (bool, error) {
		confirmed++
		current, _ := os.ReadFile(path)
		if !f.CanRepair() || string(current) != string(original) {
			t.Fatal("file changed before repair confirmation")
		}
		return true, nil
	})
	if err != nil || confirmed != 1 {
		t.Fatal(err, confirmed)
	}
	backups, _ := filepath.Glob(path + ".before-repair-*")
	if len(backups) != 1 {
		t.Fatal("original backup is missing", backups)
	}
	backup, _ := os.ReadFile(backups[0])
	if string(backup) != string(original) {
		t.Fatal("backup does not contain the exact original")
	}
	stages, _ := filepath.Glob(filepath.Join(root, ".configuration-repair-*"))
	if len(stages) != 0 {
		t.Fatal("repair left temporary files", stages)
	}
}

func TestRecoveryCannotRepairAfterDeclineChangeOrCancellation(t *testing.T) {
	for _, mode := range []string{"declined", "changed", "cancelled", "directory", "dialog_error"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := brokenFile(t, root, "metadata.json")
			original, _ := os.ReadFile(path)
			failure := InvalidWithRepair(path, original, []byte(`{}`), errors.New("invalid metadata"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := Recover(ctx, root, func() error { return failure }, func(*Failure) (bool, error) {
				switch mode {
				case "declined":
					return false, nil
				case "changed":
					if err := os.WriteFile(path, []byte("newer contents"), 0o600); err != nil {
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
				t.Fatal("unsafe repair succeeded")
			}
			if mode != "directory" {
				current, _ := os.ReadFile(path)
				want := string(original)
				if mode == "changed" {
					want = "newer contents"
				}
				if string(current) != want {
					t.Fatal("unconfirmed contents were changed")
				}
			}
			backups, _ := filepath.Glob(path + ".before-repair-*")
			if len(backups) != 0 {
				t.Fatal("unconfirmed repair created a backup")
			}
		})
	}
}

func TestRepairWriteFailureRestoresOriginalOrRetainsBackup(t *testing.T) {
	for _, failRestore := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "backup retained"}[failRestore], func(t *testing.T) {
			root := t.TempDir()
			path := brokenFile(t, root, "metadata.json")
			stage := filepath.Join(root, "candidate.json")
			backup := path + ".before-repair-test"
			if err := os.WriteFile(stage, []byte(`{}`), 0o600); err != nil {
				t.Fatal(err)
			}
			err := activateRepair(path, stage, backup, func(from, to string) error {
				if from == stage || (failRestore && from == backup) {
					return os.ErrPermission
				}
				return os.Rename(from, to)
			})
			if !errors.Is(err, os.ErrPermission) {
				t.Fatal("write failure was hidden", err)
			}
			retained := path
			if failRestore {
				retained = backup
			}
			contents, err := os.ReadFile(retained)
			if err != nil || string(contents) != "broken configuration" {
				t.Fatal("write failure lost the original", err)
			}
		})
	}
}
