//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"jeemi/internal/platform/helperstate"
)

func serviceName(uid uint32) string      { return fmt.Sprintf("jeemi-authorizer-%d.service", uid) }
func serviceDirectory(uid uint32) string { return fmt.Sprintf("/usr/libexec/jeemi-authorizer-%d", uid) }
func serviceBinary(uid uint32) string    { return filepath.Join(serviceDirectory(uid), HelperName) }
func serviceSocket(uid uint32) string {
	return fmt.Sprintf("/run/jeemi-authorizer-%d/control.sock", uid)
}
func serviceUnit(uid uint32) []byte {
	return []byte(fmt.Sprintf("# Jeemi authorization service v1; UID %d\n[Unit]\nDescription=Jeemi Authorization Helper (%d)\nAfter=polkit.service\nStartLimitIntervalSec=60\nStartLimitBurst=3\n[Service]\nType=simple\nExecStart=%s --service %d\nRestart=on-failure\nRestartSec=3\nRuntimeDirectory=jeemi-authorizer-%d\nRuntimeDirectoryMode=0755\nUMask=0077\nKillMode=control-group\nTimeoutStopSec=10\n[Install]\nWantedBy=multi-user.target\n", uid, uid, serviceBinary(uid), uid, uid))
}

const legacyHelperName = "jeemi-core-authorizer"

func legacyServiceUnit(uid uint32) []byte {
	return bytes.ReplaceAll(serviceUnit(uid), []byte("/"+HelperName+" --service"), []byte("/"+legacyHelperName+" --service"))
}

func ownedServiceUnit(data []byte, uid uint32) bool {
	return bytes.Equal(data, serviceUnit(uid)) || bytes.Equal(data, legacyServiceUnit(uid))
}

func systemctl(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/systemctl", append([]string{"--system", "--no-ask-password"}, args...)...)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C"}
	command.WaitDelay = time.Second
	var output limitedOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	err := command.Run()
	if err != nil && !(len(args) > 0 && args[0] == "show" && strings.TrimSpace(output.String()) == "not-found") {
		return nil, failure("authorization_service_failed")
	}
	return output.Bytes(), nil
}

func ServiceStatus(ctx context.Context) (helperstate.State, error) {
	uid := uint32(os.Getuid())
	f := helperstate.Facts{}
	unit := filepath.Join("/etc/systemd/system", serviceName(uid))
	for _, p := range []string{unit, serviceDirectory(uid)} {
		if _, err := os.Lstat(p); err == nil {
			f.Present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return helperstate.State{}, failure("authorization_inspection_failed")
		}
	}
	load, err := systemctl(ctx, "show", serviceName(uid), "--property=LoadState", "--value")
	if err != nil {
		return helperstate.State{}, failure("authorization_inspection_failed")
	}
	f.Registered = strings.TrimSpace(string(load)) != "not-found" && strings.TrimSpace(string(load)) != ""
	if !f.Present && !f.Registered {
		return helperstate.Evaluate(f, helperSHA256), nil
	}
	unitDir, err := openPolicyDirectory("/etc/systemd/system", 0)
	if err == nil {
		defer unitDir.Close()
		data, _, e := readPolicyFile(unitDir, serviceName(uid), 0)
		if e == nil && bytes.Equal(data, legacyServiceUnit(uid)) {
			return helperstate.State{Present: true, Code: "authorization_update_required", Action: "update"}, nil
		}
		f.Trusted = e == nil && bytes.Equal(data, serviceUnit(uid))
	}
	if f.Trusted {
		f.Digest, err = installedServiceDigest(uid)
		f.Trusted = err == nil
	}
	if f.Trusted {
		reply, e := serviceCall(ctx, serviceRequest{Operation: "status"})
		f.Reachable = e == nil
		f.Protocol = reply.Protocol
		if e == nil && reply.Digest != f.Digest {
			f.Trusted = false
		}
	}
	return helperstate.Evaluate(f, helperSHA256), nil
}

func installedServiceDigest(uid uint32) (string, error) {
	directory, err := openPolicyDirectory(serviceDirectory(uid), 0)
	if err != nil {
		return "", err
	}
	defer directory.Close()
	fd, err := unix.Openat(int(directory.Fd()), HelperName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), HelperName)
	defer file.Close()
	var stat unix.Stat_t
	if unix.Fstat(fd, &stat) != nil || stat.Uid != 0 || stat.Nlink != 1 || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0022 != 0 || stat.Size > maxBinarySize {
		return "", failure("authorization_identity_failed")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func InstallService(ctx context.Context) error {
	data, _ := json.Marshal(helperRequest{Operation: "install-service"})
	if err := runSealedHelper(ctx, data); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		state, err := ServiceStatus(ctx)
		if err == nil && state.Ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return failure("authorization_service_failed")
}

func ensureRootChild(parent, name string) (*os.File, error) {
	dir, err := openPolicyDirectory(parent, 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	err = unix.Mkdirat(int(dir.Fd()), name, 0755)
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return nil, err
	}
	return openPolicyDirectory(filepath.Join(parent, name), 0)
}

func installService(ctx context.Context, uid uint32) error {
	if uid == 0 {
		return failure("authorization_identity_failed")
	}
	lib, err := ensureRootChild("/usr", "libexec")
	if err != nil {
		return err
	}
	lib.Close()
	dir, err := ensureRootChild("/usr/libexec", filepath.Base(serviceDirectory(uid)))
	if err != nil {
		return err
	}
	defer dir.Close()
	if unix.Flock(int(dir.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return failure("authorization_busy")
	}
	defer unix.Flock(int(dir.Fd()), unix.LOCK_UN)
	units, err := openPolicyDirectory("/etc/systemd/system", 0)
	if err != nil {
		return err
	}
	defer units.Close()
	old, _, err := readPolicyFile(units, serviceName(uid), 0)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !ownedServiceUnit(old, uid) {
		return failure("authorization_install_conflict")
	}
	// Copy the currently running sealed, GUI-verified executable, not a path
	// supplied by the unprivileged caller while the password dialog was open.
	source, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || info.Size() < 64 || info.Size() > maxBinarySize {
		return failure("authorization_identity_failed")
	}
	tmp, err := os.CreateTemp(serviceDirectory(uid), ".install-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = io.CopyN(tmp, source, info.Size()); err == nil {
		err = tmp.Chmod(0755)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	backup := ""
	result := helperstate.Replace(func() error { return nil }, func() error {
		if old != nil {
			_, e := systemctl(ctx, "stop", serviceName(uid))
			return e
		}
		return nil
	}, func() error {
		if _, e := os.Lstat(serviceBinary(uid)); e == nil {
			if _, e = installedServiceDigest(uid); e != nil {
				return e
			}
			placeholder, e := os.CreateTemp(serviceDirectory(uid), ".previous-")
			if e != nil {
				return e
			}
			name := placeholder.Name()
			placeholder.Close()
			os.Remove(name)
			if e = os.Rename(serviceBinary(uid), name); e != nil {
				return e
			}
			backup = name
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if e := os.Rename(tmp.Name(), serviceBinary(uid)); e != nil {
			return e
		}
		if e := writeRootUnit(units, serviceName(uid), serviceUnit(uid)); e != nil {
			return e
		}
		if _, e := systemctl(ctx, "daemon-reload"); e != nil {
			return e
		}
		_, e := systemctl(ctx, "enable", "--now", serviceName(uid))
		return e
	}, func() error {
		expected, e := installedServiceDigest(uid)
		if e != nil {
			return e
		}
		for i := 0; i < 30; i++ {
			reply, e := serviceCallFor(ctx, uid, serviceRequest{Operation: "status"})
			if e == nil && reply.Digest == expected {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		return failure("authorization_service_failed")
	}, func() error {
		recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, e := systemctl(recovery, "stop", serviceName(uid)); e != nil {
			return e
		}
		if backup != "" {
			if e := os.Rename(backup, serviceBinary(uid)); e != nil {
				return e
			}
		}
		if old != nil {
			if e := writeRootUnit(units, serviceName(uid), old); e != nil {
				return e
			}
			if _, e := systemctl(recovery, "daemon-reload"); e != nil {
				return e
			}
			_, e := systemctl(recovery, "start", serviceName(uid))
			return e
		}
		_, e := systemctl(recovery, "disable", serviceName(uid))
		return e
	})
	if result == nil && backup != "" {
		os.Remove(backup)
	}
	if result == nil {
		// The old process has stopped and the new filename has answered. Retire
		// only the fixed root-owned legacy executable, never a directory tree.
		var stat unix.Stat_t
		if e := unix.Fstatat(int(dir.Fd()), legacyHelperName, &stat, unix.AT_SYMLINK_NOFOLLOW); e == nil {
			if stat.Uid != 0 || stat.Nlink != 1 || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0022 != 0 {
				return failure("authorization_install_conflict")
			}
			return unix.Unlinkat(int(dir.Fd()), legacyHelperName, 0)
		} else if !errors.Is(e, unix.ENOENT) {
			return e
		}
	}
	return result
}

func writeRootUnit(directory *os.File, name string, data []byte) error {
	file, err := os.CreateTemp(directory.Name(), ".jeemi-unit-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Chmod(0644)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(directory.Name(), name))
}

func removeService(ctx context.Context, uid uint32, stop bool) error {
	dir, err := openPolicyDirectory(serviceDirectory(uid), 0)
	var files []string
	if err == nil {
		defer dir.Close()
		if unix.Flock(int(dir.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
			return failure("authorization_busy")
		}
		defer unix.Flock(int(dir.Fd()), unix.LOCK_UN)
		files, err = serviceRemovalFiles(dir)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	units, err := openPolicyDirectory("/etc/systemd/system", 0)
	if err != nil {
		return err
	}
	defer units.Close()
	data, _, err := readPolicyFile(units, serviceName(uid), 0)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && !ownedServiceUnit(data, uid) {
		return failure("authorization_install_conflict")
	}
	if data != nil {
		if stop {
			if _, err = systemctl(ctx, "stop", serviceName(uid)); err != nil {
				return err
			}
		}
		if _, err = systemctl(ctx, "disable", serviceName(uid)); err != nil {
			return err
		}
		if err = unix.Unlinkat(int(units.Fd()), serviceName(uid), 0); err != nil && !errors.Is(err, unix.ENOENT) {
			return err
		}
	}
	if dir != nil {
		for _, name := range files {
			if err = unix.Unlinkat(int(dir.Fd()), name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
				return err
			}
		}
		if err = os.Remove(serviceDirectory(uid)); err != nil {
			return failure("authorization_cleanup_incomplete")
		}
	}
	_, err = systemctl(ctx, "daemon-reload")
	return err
}

// Include only installer-owned files so an interrupted replacement can be
// cleaned without recursively removing an arbitrary directory tree.
func serviceRemovalFiles(directory *os.File) ([]string, error) {
	entries, err := directory.ReadDir(-1)
	if err != nil || len(entries) > 256 {
		return nil, failure("authorization_cleanup_incomplete")
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !serviceOwnedName(name) {
			return nil, failure("authorization_install_conflict")
		}
		var info unix.Stat_t
		if unix.Fstatat(int(directory.Fd()), name, &info, unix.AT_SYMLINK_NOFOLLOW) != nil || info.Uid != 0 || info.Nlink != 1 || info.Mode&unix.S_IFMT != unix.S_IFREG || info.Mode&0022 != 0 {
			return nil, failure("authorization_install_conflict")
		}
		names = append(names, name)
	}
	return names, nil
}

func serviceOwnedName(name string) bool {
	if name == HelperName || name == legacyHelperName {
		return true
	}
	for _, prefix := range []string{".install-", ".previous-"} {
		if suffix, ok := strings.CutPrefix(name, prefix); ok && len(suffix) > 0 && len(suffix) <= 20 {
			if _, err := strconv.ParseUint(suffix, 10, 64); err == nil && !strings.HasPrefix(suffix, "+") {
				return true
			}
		}
	}
	return false
}

func serviceUID(value string) (uint32, error) {
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != value {
		return 0, failure("authorization_identity_failed")
	}
	return uint32(n), nil
}
