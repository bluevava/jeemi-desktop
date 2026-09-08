package appupdate

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"jeemi/internal/platform/updateprocess"
)

type installJob struct {
	ID            string `json:"id"`
	Executable    string `json:"executable"`
	CurrentSHA256 string `json:"currentSHA256"`
	Version       string `json:"version"`
}

type replacement struct {
	root, stage         string
	items               []string
	backedUp, installed []string
	rename              func(string, string) error
}

func prepareReplacement(ctx context.Context, jobRoot string, job installJob, t target) (*replacement, error) {
	root, err := t.installRoot(job.Executable)
	if err != nil || !idPattern.MatchString(job.ID) {
		return nil, ErrLocation
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || real != root {
		return nil, ErrLocation
	}
	digest, _, err := hashFile(job.Executable)
	if err != nil || digest != job.CurrentSHA256 {
		return nil, ErrLocation
	}
	source := filepath.Join(jobRoot, "extracted", t.directory)
	if err = verifyPackage(ctx, source, job.Version, t); err != nil {
		return nil, err
	}
	stage := filepath.Join(root, ".jeemi-update-"+job.ID)
	if err = os.Mkdir(stage, 0700); err != nil {
		return nil, ErrLocation
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(stage)
		}
	}()
	r := &replacement{root: root, stage: stage, items: t.installItems(), rename: updateprocess.Rename}
	for _, part := range []string{"new", "old", "failed"} {
		if os.Mkdir(filepath.Join(stage, part), 0700) != nil {
			return nil, ErrLocation
		}
	}
	for _, name := range r.items {
		if info, err := os.Lstat(filepath.Join(root, name)); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrLocation
		} else if err != nil && !os.IsNotExist(err) {
			return nil, ErrLocation
		}
		if err = copyTree(filepath.Join(source, name), filepath.Join(stage, "new", name)); err != nil {
			return nil, ErrLocation
		}
	}
	if t.platform == "macos" {
		for _, name := range []string{"release.json", "SHA256SUMS"} {
			if copyTree(filepath.Join(source, name), filepath.Join(stage, "new", name)) != nil {
				return nil, ErrLocation
			}
		}
	}
	if err = verifyPackage(ctx, filepath.Join(stage, "new"), job.Version, t); err != nil {
		return nil, err
	}
	if t.platform == "macos" && updateprocess.VerifyBundle(ctx, filepath.Join(stage, "new", "Jeemi.app")) != nil {
		return nil, ErrPackage
	}
	ok = true
	return r, nil
}

// Rename within the install filesystem. Only the known release files (or the
// complete macOS bundle) are replaced; other files in that folder are retained.
func (r *replacement) apply() error {
	for _, name := range r.items {
		filename := filepath.Join(r.root, name)
		if _, err := os.Lstat(filename); err == nil {
			if r.rename(filename, filepath.Join(r.stage, "old", name)) != nil {
				return r.failed()
			}
			r.backedUp = append(r.backedUp, name)
		} else if !os.IsNotExist(err) {
			return r.failed()
		}
		if r.rename(filepath.Join(r.stage, "new", name), filename) != nil {
			return r.failed()
		}
		r.installed = append(r.installed, name)
	}
	return nil
}

func (r *replacement) failed() error {
	if r.rollback() != nil {
		return ErrRollback
	}
	return ErrReplace
}

func (r *replacement) rollback() error {
	failed := false
	for i := len(r.installed) - 1; i >= 0; i-- {
		name := r.installed[i]
		if r.rename(filepath.Join(r.root, name), filepath.Join(r.stage, "failed", name)) != nil {
			failed = true
		}
	}
	for i := len(r.backedUp) - 1; i >= 0; i-- {
		name := r.backedUp[i]
		if r.rename(filepath.Join(r.stage, "old", name), filepath.Join(r.root, name)) != nil {
			failed = true
		}
	}
	if failed {
		return ErrRollback
	}
	r.installed = nil
	r.backedUp = nil
	return nil
}

func (r *replacement) cleanup() { _ = os.RemoveAll(r.stage) }

func copyTree(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, destination)
	}
	if info.IsDir() {
		if err = os.Mkdir(destination, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err = copyTree(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return ErrPackage
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err = io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}
