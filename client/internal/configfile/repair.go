package configfile

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func repairConfirmed(ctx context.Context, root string, failure *Failure) error {
	if err := recheckConfirmed(root, failure); err != nil {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(failure.Path), ".configuration-repair-*")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	if err := stage.Chmod(0o600); err != nil {
		stage.Close()
		return err
	}
	if _, err := stage.Write(failure.repair); err != nil {
		stage.Close()
		return err
	}
	if err := stage.Sync(); err != nil {
		stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := recheckConfirmed(root, failure); err != nil {
		return err
	}
	// Retain the exact original beside the repaired document. The backup is not
	// an input to normal reads, and never overwrites an earlier recovery copy.
	backup := failure.Path + ".before-repair-" + rand.Text()
	return activateRepair(failure.Path, stage.Name(), backup, os.Rename)
}

func activateRepair(path, stage, backup string, rename func(string, string) error) error {
	if err := rename(path, backup); err != nil {
		return err
	}
	if err := rename(stage, path); err != nil {
		if restoreErr := rename(backup, path); restoreErr != nil {
			return errors.Join(fmt.Errorf("activate repaired configuration: %w", err), fmt.Errorf("restore original configuration: %w", restoreErr))
		}
		return err
	}
	return nil
}
