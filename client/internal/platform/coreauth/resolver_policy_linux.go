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
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

var resolverActions = [...]string{
	"org.freedesktop.resolve1.set-domains",
	"org.freedesktop.resolve1.set-default-route",
	"org.freedesktop.resolve1.set-dns-servers",
	"org.freedesktop.resolve1.revert",
}

type resolverPolicy struct {
	name    string
	rule    []byte
	receipt []byte
}

func makeResolverPolicy(uid uint32, username string) resolverPolicy {
	quoted, _ := json.Marshal(username)
	var rule strings.Builder
	fmt.Fprintf(&rule, "// Jeemi network authorization v1; administrator-approved UID %d.\n", uid)
	fmt.Fprintf(&rule, "polkit.addRule(function (action, subject) {\n    if (subject.user !== %s || action.lookup(\"interface\") !== \"JeemiTun\") {\n        return;\n    }\n    switch (action.id) {\n", quoted)
	for _, action := range resolverActions {
		fmt.Fprintf(&rule, "    case %q:\n", action)
	}
	rule.WriteString("        return polkit.Result.YES;\n    }\n});\n")
	data := []byte(rule.String())
	digest := sha256.Sum256(data)
	return resolverPolicy{
		name: fmt.Sprintf("90-jeemi-network-v1-%d", uid), rule: data,
		receipt: []byte("Jeemi network authorization v1 verified\n" + hex.EncodeToString(digest[:]) + "\n"),
	}
}

// The owner argument is injectable only in package tests; production always
// uses UID 0. A readable, exact rule and verification receipt are both required.
func (p resolverPolicy) check(directory string, owner uint32) error {
	dir, err := openPolicyDirectory(directory, owner)
	if err != nil {
		return failure("linux_resolver_authorization_unavailable")
	}
	defer dir.Close()
	for _, entry := range p.entries() {
		data, _, err := readPolicyFile(dir, entry.name, owner)
		if errors.Is(err, os.ErrNotExist) {
			return failure("linux_core_permission_required")
		}
		if err != nil || !bytes.Equal(data, entry.data) {
			return failure("linux_resolver_authorization_conflict")
		}
	}
	return nil
}

type policyEntry struct {
	name string
	data []byte
}

func (p resolverPolicy) entries() []policyEntry {
	return []policyEntry{{p.name + ".rules", p.rule}, {p.name + ".verified", p.receipt}}
}

func (p resolverPolicy) install(ctx context.Context, directory string, owner uint32, verify func(context.Context) error) error {
	dir, err := openPolicyDirectory(directory, owner)
	if err != nil {
		return failure("linux_resolver_authorization_unavailable")
	}
	defer dir.Close()
	if unix.Flock(int(dir.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return failure("linux_core_authorization_busy")
	}
	defer unix.Flock(int(dir.Fd()), unix.LOCK_UN)
	// Validate both existing names before writing either one. Never overwrite an
	// administrator's different rule or accept a receipt from another rule.
	for _, entry := range p.entries() {
		data, _, err := readPolicyFile(dir, entry.name, owner)
		if err != nil && !errors.Is(err, os.ErrNotExist) || err == nil && !bytes.Equal(data, entry.data) {
			return failure("linux_resolver_authorization_conflict")
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	created, err := writePolicyFile(dir, p.name+".rules", p.rule, owner)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success && created != nil {
			// Remove only the file we installed, preserving any administrator edit.
			_, current, err := readPolicyFile(dir, p.name+".rules", owner)
			if err == nil && os.SameFile(created, current) {
				_ = unix.Unlinkat(int(dir.Fd()), p.name+".rules", 0)
			}
		}
	}()
	// Check the actual four decisions as root, for the original GUI's identity.
	// A conflicting earlier administrator rule must fail without four dialogs.
	if err := verify(ctx); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := writePolicyFile(dir, p.name+".verified", p.receipt, owner); err != nil {
		return err
	}
	success = true
	return p.check(directory, owner)
}

func openPolicyDirectory(path string, owner uint32) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, os.ErrPermission
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		_ = unix.Close(fd)
		if err != nil {
			return nil, err
		}
		fd = next
		var info unix.Stat_t
		if unix.Fstat(fd, &info) != nil || (info.Uid != 0 && info.Uid != owner) ||
			(info.Mode&0o022 != 0 && !(i < len(parts)-1 && info.Uid == 0 && info.Mode&unix.S_ISVTX != 0)) {
			_ = unix.Close(fd)
			return nil, os.ErrPermission
		}
	}
	return os.NewFile(uintptr(fd), path), nil
}

func readPolicyFile(dir *os.File, name string, owner uint32) ([]byte, os.FileInfo, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var stat unix.Stat_t
	if unix.Fstat(fd, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != owner || stat.Mode&0o022 != 0 || stat.Nlink != 1 || stat.Size > 8192 {
		return nil, nil, os.ErrPermission
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if len(data) > 8192 {
		return nil, nil, os.ErrPermission
	}
	return data, info, err
}

func writePolicyFile(dir *os.File, name string, data []byte, owner uint32) (os.FileInfo, error) {
	current, _, err := readPolicyFile(dir, name, owner)
	if err == nil && bytes.Equal(current, data) {
		return nil, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, failure("linux_resolver_authorization_conflict")
	}
	// Create through the already-verified directory FD, including on systems
	// where /usr/share can be replaced by an administrator during authentication.
	file, err := os.CreateTemp(fmt.Sprintf("/proc/self/fd/%d", dir.Fd()), ".jeemi-policy-*")
	if err != nil {
		return nil, failure("linux_resolver_authorization_failed")
	}
	defer file.Close()
	temporary := filepath.Base(file.Name())
	defer unix.Unlinkat(int(dir.Fd()), temporary, 0)
	if _, err = file.Write(data); err == nil {
		err = file.Chmod(0o644)
	}
	if err == nil {
		err = file.Sync()
	}
	if err == nil {
		err = unix.Renameat2(int(dir.Fd()), temporary, int(dir.Fd()), name, unix.RENAME_NOREPLACE)
	}
	if err != nil {
		return nil, failure("linux_resolver_authorization_failed")
	}
	if err = dir.Sync(); err != nil {
		return nil, failure("linux_resolver_authorization_failed")
	}
	return file.Stat()
}
