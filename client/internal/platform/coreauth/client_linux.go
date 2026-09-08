//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"jeemi/internal/platform/requirements"

	"golang.org/x/sys/unix"
)

// Authorize is called only from an explicit start/restart action. Automatic
// configuration updates and process recovery only perform read-only preflight.
func Authorize(ctx context.Context, target Target) error {
	state, err := ServiceStatus(ctx)
	if err != nil {
		return err
	}
	if !state.Ready {
		return failure(state.Code)
	}
	if err = Check(target.ExecutablePath, 0); err == nil {
		return nil
	}
	_, err = serviceCall(ctx, serviceRequest{Operation: "authorize", Target: &target})
	if err != nil {
		return err
	}
	return Check(target.ExecutablePath, 0)
}

func authorize(ctx context.Context, target Target, check func(string, int) error) error {
	if err := check(target.ExecutablePath, 0); err == nil {
		return nil
	} else if requirements.Code(err, "") != "linux_core_permission_required" {
		return err
	}
	if os.Geteuid() == 0 {
		return failure("linux_core_authorization_unavailable")
	}
	// Detect unsupported resolver environments before presenting a password UI.
	if resolverAvailable() {
		if err := checkResolverSupport(ctx); err != nil {
			return err
		}
	}
	file, err := openCore(target, uint32(os.Getuid()))
	if err != nil {
		return err
	}
	defer file.Close()
	if err := verifyCore(ctx, file, target); err != nil {
		return err
	}
	input, _ := json.Marshal(target)
	if err := runSealedHelper(ctx, input); err != nil {
		return err
	}
	// Verify the same installed file and the actual persisted kernel attributes.
	current, err := openCore(target, uint32(os.Getuid()))
	if err != nil {
		return err
	}
	defer current.Close()
	before, beforeErr := file.Stat()
	after, statErr := current.Stat()
	if beforeErr != nil || statErr != nil || !os.SameFile(before, after) {
		return failure("linux_core_integrity_failed")
	}
	if err := verifyCore(ctx, current, target); err != nil {
		return err
	}
	return check(target.ExecutablePath, 0)
}

func runSealedHelper(ctx context.Context, input []byte) error {
	if os.Geteuid() == 0 {
		return failure("linux_core_authorization_unavailable")
	}
	executable, err := os.Executable()
	if err != nil {
		return failure("linux_core_authorizer_missing")
	}
	helper, err := prepareHelper(ctx, filepath.Join(filepath.Dir(executable), HelperName))
	if err != nil {
		return err
	}
	defer helper.Close()
	// Use the system pkexec, never a PATH-provided executable. No password is
	// requested by or transmitted through Jeemi's WebView/stdin protocol.
	const pkexec = "/usr/bin/pkexec"
	if info, err := os.Stat(pkexec); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return failure("linux_core_authorization_unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// Execute the verified sealed bytes. A portable release directory is user
	// writable and its helper path could otherwise change during the dialog.
	helperPath := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), helper.Fd())
	command := exec.CommandContext(ctx, pkexec, "--disable-internal-agent", helperPath)
	command.Stdin = bytes.NewReader(input)
	var output limitedOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = 2 * time.Second
	// pkexec uses Pdeathsig while waiting for Polkit; keep its creator alive.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err = command.Run()
	if err := authorizationResult(ctx.Err(), err, output.Bytes()); err != nil {
		return err
	}
	return nil
}

func authorizationResult(contextErr, runErr error, output []byte) error {
	if contextErr != nil {
		if errors.Is(contextErr, context.Canceled) {
			return ErrCancelled
		}
		return failure("linux_core_authorization_timeout")
	}
	var exitError *exec.ExitError
	if errors.As(runErr, &exitError) && exitError.ExitCode() == 124 {
		return failure("linux_core_authorization_timeout")
	}
	if errors.As(runErr, &exitError) && exitError.ExitCode() == 126 {
		return ErrCancelled
	}
	if errors.As(runErr, &exitError) && exitError.ExitCode() == 127 {
		return failure("linux_core_authentication_failed")
	}
	var result response
	if json.Unmarshal(output, &result) != nil {
		return failure("linux_core_authorization_failed")
	}
	if runErr == nil && result.Code == "authorized" {
		return nil
	}
	switch result.Code {
	case "authorization_busy", "authorization_service_failed", "authorization_identity_failed", "authorization_install_conflict", "authorization_cleanup_incomplete":
		return failure(result.Code)
	case "linux_core_cleanup_conflict", "linux_core_cleanup_incomplete", "linux_core_integrity_failed", "linux_core_authorization_unsafe_path", "linux_core_authorization_busy", "linux_core_authorization_filesystem", "linux_resolver_authorization_conflict", "linux_resolver_authorization_unavailable", "linux_resolver_authorization_failed":
		return failure(result.Code)
	default:
		return failure("linux_core_authorization_failed")
	}
}

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(data []byte) (int, error) {
	if left := 4096 - b.Len(); left > 0 {
		_, _ = b.Buffer.Write(data[:min(left, len(data))])
	}
	return len(data), nil
}

func prepareHelper(ctx context.Context, path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || len(helperSHA256) != 64 {
		return nil, failure("linux_core_authorizer_missing")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, failure("linux_core_authorizer_missing")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, failure("linux_core_authorizer_missing")
	}
	// Portable mounts can report 0777 or omit executable bits. We only read
	// this file: pkexec runs the sealed, digest-verified copy below, whose
	// permissions and bytes are independent of the source mount.
	flags := unix.MFD_CLOEXEC | unix.MFD_ALLOW_SEALING
	fd, err := unix.MemfdCreate(HelperName, flags|unix.MFD_EXEC)
	if errors.Is(err, unix.EINVAL) {
		fd, err = unix.MemfdCreate(HelperName, flags) // Linux before MFD_EXEC
	}
	if err != nil {
		return nil, failure("linux_core_authorizer_missing")
	}
	sealed := os.NewFile(uintptr(fd), HelperName)
	failed := func() (*os.File, error) { _ = sealed.Close(); return nil, failure("linux_core_authorizer_missing") }
	if info.Size() < 64 || info.Size() > maxBinarySize {
		return failed()
	}
	if _, err := io.CopyN(sealed, file, info.Size()); err != nil {
		return failed()
	}
	if err := sealed.Chmod(0o500); err != nil {
		return failed()
	}
	if _, err := unix.FcntlInt(sealed.Fd(), unix.F_ADD_SEALS, unix.F_SEAL_WRITE|unix.F_SEAL_GROW|unix.F_SEAL_SHRINK|unix.F_SEAL_SEAL); err != nil {
		return failed()
	}
	if err := verifyCore(ctx, sealed, Target{SHA256: helperSHA256, Size: info.Size()}); err != nil {
		return failed()
	}
	return sealed, nil
}
