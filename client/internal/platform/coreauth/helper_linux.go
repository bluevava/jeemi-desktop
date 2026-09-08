//go:build linux

package coreauth

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"jeemi/internal/platform/requirements"

	"golang.org/x/sys/unix"
)

// RunHelper is the administrator-approved, CGO-free installation entry point.
// It manages only the caller's fixed service, verified core capabilities and
// narrowly scoped resolver policy, never arbitrary commands or root paths.
func RunHelper(input io.Reader, output io.Writer) int {
	uid, err := strconv.ParseUint(os.Getenv("PKEXEC_UID"), 10, 32)
	if os.Geteuid() != 0 || os.Getuid() != 0 || err != nil || uid == 0 || len(os.Args) != 1 {
		_ = json.NewEncoder(output).Encode(response{Code: "linux_core_authentication_failed"})
		return 1
	}
	// Bound the entire privileged lifetime, including input and filesystem I/O
	// which can otherwise block independently of a context deadline.
	watchdog := time.AfterFunc(20*time.Second, func() { os.Exit(124) })
	defer watchdog.Stop()
	parent := os.Getppid()
	if parent <= 1 || unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(syscall.SIGTERM), 0, 0, 0) != nil || os.Getppid() != parent {
		_ = json.NewEncoder(output).Encode(response{Code: "linux_core_authentication_failed"})
		return 1
	}
	request, err := decodeHelperRequest(input)
	if err == nil {
		lifetime := 20 * time.Second
		if request.Operation == "install-service" || request.Operation == "remove-service" {
			lifetime = 90 * time.Second
			watchdog.Reset(lifetime)
		}
		ctx, cancel := context.WithTimeout(context.Background(), lifetime)
		defer cancel()
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGIO)
		defer stop()
		if request.Operation == "install-service" {
			err = installService(ctx, uint32(uid))
		} else if request.Operation == "remove-service" {
			err = cleanupForUID(ctx, request.Targets, uint32(uid))
			if err == nil {
				err = removeService(ctx, uint32(uid), true)
			}
		} else if request.Operation == "remove" {
			for _, target := range request.Targets {
				if ctx.Err() != nil {
					err = ctx.Err()
					break
				}
				if err = revoke(target, uint32(uid), kernelGrantOps{}); err != nil {
					break
				}
			}
			if err == nil {
				var policy resolverPolicy
				policy, err = resolverPolicyFor(uint32(uid))
				if err == nil {
					err = policy.remove(resolverRulesDirectory, 0)
				}
			}
		} else {
			err = grant(ctx, request.Target, uint32(uid), kernelGrantOps{})
			if err == nil {
				err = authorizeResolver(ctx, uint32(uid), parent)
			}
		}
	}
	code := "authorized"
	exitCode := 0
	if err != nil {
		code = requirements.Code(err, "linux_core_authorization_failed")
		exitCode = 1
	}
	_ = json.NewEncoder(output).Encode(response{Code: code})
	return exitCode
}

type helperRequest struct {
	Target
	Operation string   `json:"operation,omitempty"`
	Targets   []Target `json:"targets,omitempty"`
}

func decodeHelperRequest(input io.Reader) (helperRequest, error) {
	var request helperRequest
	data, err := io.ReadAll(io.LimitReader(input, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return request, failure("linux_core_integrity_failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&request) != nil || decoder.Decode(&extra) != io.EOF || len(request.Targets) > 256 {
		return request, failure("linux_core_integrity_failed")
	}
	if request.Operation == "install-service" {
		if request.Target != (Target{}) || request.Targets != nil {
			return request, failure("linux_core_integrity_failed")
		}
	} else if request.Operation == "remove" || request.Operation == "remove-service" {
		if request.Target != (Target{}) || request.Targets == nil {
			return request, failure("linux_core_integrity_failed")
		}
	} else if request.Operation != "" || request.Targets != nil {
		return request, failure("linux_core_integrity_failed")
	}
	return request, nil
}

func decodeTarget(input io.Reader) (Target, error) {
	data, err := io.ReadAll(io.LimitReader(input, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return Target{}, failure("linux_core_integrity_failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var target Target
	if decoder.Decode(&target) != nil {
		return Target{}, failure("linux_core_integrity_failed")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Target{}, failure("linux_core_integrity_failed")
	}
	return target, nil
}

type grantOps interface {
	filesystemFlags(int) (int64, error)
	lease(int) error
	release(int)
	leaseIntact(int) bool
	setCapabilities(int, []byte) error
	capabilities(int) ([]byte, error)
}

func grant(ctx context.Context, target Target, uid uint32, ops grantOps) error {
	file, err := openCore(target, uid)
	if err != nil {
		return err
	}
	defer file.Close()
	fd := int(file.Fd())
	flags, err := ops.filesystemFlags(fd)
	if err != nil || flags&(unix.ST_NOSUID|unix.ST_NOEXEC|unix.ST_RDONLY) != 0 {
		return failure("linux_core_authorization_filesystem")
	}
	// A read lease prevents writable opens/truncation across hash verification
	// and the xattr write. SIGIO cancels the grant if a writer requests a break.
	if err := ops.lease(fd); err != nil {
		return failure("linux_core_authorization_busy")
	}
	defer ops.release(fd)
	if err := verifyCore(ctx, file, target); err != nil {
		return err
	}
	if ctx.Err() != nil || !ops.leaseIntact(fd) {
		return failure("linux_core_authorization_busy")
	}
	caps := networkCapabilities()
	if err := ops.setCapabilities(fd, caps); err != nil {
		return failure("linux_core_authorization_filesystem")
	}
	actual, err := ops.capabilities(fd)
	if err != nil || !requirements.HasNetworkCapabilities(actual) {
		return failure("linux_core_authorization_failed")
	}
	return nil
}

func networkCapabilities() []byte {
	value := make([]byte, 20)
	binary.LittleEndian.PutUint32(value, 0x02000001)
	binary.LittleEndian.PutUint32(value[4:], 1<<unix.CAP_NET_ADMIN|1<<unix.CAP_NET_RAW|1<<unix.CAP_NET_BIND_SERVICE)
	return value
}

type kernelGrantOps struct{}

func (kernelGrantOps) filesystemFlags(fd int) (int64, error) {
	var filesystem unix.Statfs_t
	err := unix.Fstatfs(fd, &filesystem)
	return filesystem.Flags, err
}

func (kernelGrantOps) lease(fd int) error {
	_, err := unix.FcntlInt(uintptr(fd), unix.F_SETLEASE, unix.F_RDLCK)
	return err
}
func (kernelGrantOps) release(fd int) {
	_, _ = unix.FcntlInt(uintptr(fd), unix.F_SETLEASE, unix.F_UNLCK)
}
func (kernelGrantOps) leaseIntact(fd int) bool {
	value, err := unix.FcntlInt(uintptr(fd), unix.F_GETLEASE, 0)
	return err == nil && value == unix.F_RDLCK
}
func (kernelGrantOps) setCapabilities(fd int, value []byte) error {
	return unix.Fsetxattr(fd, "security.capability", value, 0)
}
func (kernelGrantOps) capabilities(fd int) ([]byte, error) {
	value := make([]byte, 24)
	count, err := unix.Fgetxattr(fd, "security.capability", value)
	if err != nil {
		return nil, err
	}
	return value[:count], nil
}
