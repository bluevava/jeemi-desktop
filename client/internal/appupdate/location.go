package appupdate

import (
	"os"
	"path/filepath"
)

// Probe the parent of the complete application, not just the executable's
// write bit: macOS updates rename the whole bundle on the same filesystem.
// This happens before downloading or stopping the current client. The worker
// repeats the checks when it stages the verified replacement.
func checkInstallLocation(executable string, t target, id string) error {
	root, err := t.installRoot(executable)
	if err != nil || !idPattern.MatchString(id) {
		return ErrLocation
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || real != root {
		return ErrLocation
	}
	probe := filepath.Join(root, ".jeemi-update-"+id)
	if err = os.Mkdir(probe, 0700); err != nil {
		return ErrLocation
	}
	if err = os.Remove(probe); err != nil {
		return ErrLocation
	}
	return nil
}
