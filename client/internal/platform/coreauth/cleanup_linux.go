//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// CleanupTargets enumerates only this data root's managed core slots, including
// old versions. Untrusted metadata cannot hide a remaining file capability.
func CleanupTargets(directory string) ([]Target, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || !strings.HasSuffix(directory, "/jeemi_data/core/mihomo") {
		return nil, failure("linux_core_authorization_unsafe_path")
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []Target{}, nil
	}
	if err != nil {
		return nil, err
	}
	targets := []Target{}
	for _, entry := range entries {
		if !versionPattern.MatchString(entry.Name()) {
			continue
		}
		p := filepath.Join(directory, entry.Name(), "linux-"+runtime.GOARCH, "mihomo")
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		targets = append(targets, Target{ExecutablePath: p})
		if len(targets) > 256 {
			return nil, failure("linux_core_authorization_failed")
		}
	}
	return targets, nil
}

// HasAuthorization deliberately checks persisted files, not helper processes.
func HasAuthorization(targets []Target) (bool, error) {
	present := false
	for _, target := range targets {
		file, err := openCore(target, uint32(os.Getuid()))
		if err != nil {
			return true, err
		}
		value, err := (kernelGrantOps{}).capabilities(int(file.Fd()))
		file.Close()
		if err != nil && !errors.Is(err, unix.ENODATA) {
			return true, err
		}
		present = present || len(value) > 0
	}
	policy, err := resolverPolicyFor(uint32(os.Getuid()))
	if err != nil {
		return true, err
	}
	for _, entry := range policy.entries() {
		_, err := os.Lstat(filepath.Join(resolverRulesDirectory, entry.name))
		if err == nil {
			present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return true, err
		}
	}
	return present, nil
}

func Cleanup(ctx context.Context, targets []Target) error {
	state, stateErr := ServiceStatus(ctx)
	if stateErr != nil {
		return stateErr
	}
	present, err := HasAuthorization(targets)
	if err != nil || (!present && !state.Present) {
		return err
	}
	if state.Ready {
		_, err = serviceCall(ctx, serviceRequest{Operation: "remove", Targets: targets})
		if err != nil {
			return err
		}
	} else {
		input, _ := json.Marshal(struct {
			Operation string   `json:"operation"`
			Targets   []Target `json:"targets"`
		}{"remove-service", targets})
		if err := runSealedHelper(ctx, input); err != nil {
			return err
		}
	}
	present, err = HasAuthorization(targets)
	if err != nil {
		return err
	}
	if present {
		return failure("linux_core_cleanup_incomplete")
	}
	// systemd keeps a deleted unit loaded until the service has returned its
	// reply and exited. Give that transition time without declaring success.
	for i := 0; i < 30; i++ {
		state, err = ServiceStatus(ctx)
		if err == nil && !state.Present {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return failure("authorization_cleanup_incomplete")
}

type revokeOps interface {
	capabilities(int) ([]byte, error)
	removeCapabilities(int) error
}

func (kernelGrantOps) removeCapabilities(fd int) error {
	return unix.Fremovexattr(fd, "security.capability")
}

func revoke(target Target, uid uint32, ops revokeOps) error {
	file, err := openCore(target, uid)
	if err != nil {
		return err
	}
	defer file.Close()
	fd := int(file.Fd())
	value, err := ops.capabilities(fd)
	if errors.Is(err, unix.ENODATA) {
		return nil
	}
	if err != nil {
		return err
	}
	// Do not remove capabilities granted independently by an administrator.
	if !bytes.Equal(value, networkCapabilities()) {
		return failure("linux_core_cleanup_conflict")
	}
	if err := ops.removeCapabilities(fd); err != nil && !errors.Is(err, unix.ENODATA) {
		return err
	}
	if _, err := ops.capabilities(fd); !errors.Is(err, unix.ENODATA) {
		return failure("linux_core_cleanup_incomplete")
	}
	return nil
}

func (p resolverPolicy) remove(directory string, owner uint32) error {
	dir, err := openPolicyDirectory(directory, owner)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer dir.Close()
	if unix.Flock(int(dir.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return failure("linux_core_authorization_busy")
	}
	defer unix.Flock(int(dir.Fd()), unix.LOCK_UN)
	for _, entry := range p.entries() {
		data, _, err := readPolicyFile(dir, entry.name, owner)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !bytes.Equal(data, entry.data) {
			return failure("linux_resolver_authorization_conflict")
		}
	}
	for _, entry := range p.entries() {
		if err := unix.Unlinkat(int(dir.Fd()), entry.name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			return err
		}
	}
	return dir.Sync()
}
