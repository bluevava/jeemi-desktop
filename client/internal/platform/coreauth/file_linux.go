//go:build linux

package coreauth

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

var versionPattern = regexp.MustCompile(`^(v[0-9]+\.[0-9]+\.[0-9]+|unknown-[a-f0-9]{12,64})$`)

// Walk relative to directory FDs, rejecting symbolic links at every component.
// The caller must own the core; shared writable directories/files and hard
// links cannot redirect authorization to another user's or system file.
func openCore(target Target, uid uint32) (*os.File, error) {
	path := target.ExecutablePath
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	n := len(parts)
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || n < 6 ||
		parts[n-6] != "jeemi_data" || parts[n-5] != "core" || parts[n-4] != "mihomo" ||
		!versionPattern.MatchString(parts[n-3]) || parts[n-2] != "linux-"+runtime.GOARCH || parts[n-1] != "mihomo" {
		return nil, failure("linux_core_authorization_unsafe_path")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, failure("linux_core_authorization_unsafe_path")
	}
	for index, part := range parts {
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
		if index != n-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(fd, part, flags, 0)
		_ = unix.Close(fd)
		if openErr != nil {
			return nil, failure("linux_core_authorization_unsafe_path")
		}
		fd = next
		var info unix.Stat_t
		if unix.Fstat(fd, &info) != nil || (info.Uid != uid && info.Uid != 0) ||
			(info.Mode&0o022 != 0 && !(index < n-6 && info.Uid == 0 && info.Mode&unix.S_ISVTX != 0)) ||
			(index == n-1 && (info.Mode&unix.S_IFMT != unix.S_IFREG || info.Uid != uid || info.Nlink != 1 || info.Mode&0o100 == 0 || info.Mode&(unix.S_ISUID|unix.S_ISGID) != 0)) {
			_ = unix.Close(fd)
			return nil, failure("linux_core_authorization_unsafe_path")
		}
	}
	return os.NewFile(uintptr(fd), "verified-mihomo"), nil
}

func verifyCore(ctx context.Context, file *os.File, target Target) error {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || target.Size < 64 || target.Size > maxBinarySize || info.Size() != target.Size {
		return failure("linux_core_integrity_failed")
	}
	expected, err := hex.DecodeString(target.SHA256)
	if err != nil || len(expected) != sha256.Size {
		return failure("linux_core_integrity_failed")
	}
	header := make([]byte, 64)
	if _, err := file.ReadAt(header, 0); err != nil || string(header[:6]) != "\x7fELF\x02\x01" {
		return failure("linux_core_integrity_failed")
	}
	machine := uint16(62)
	if runtime.GOARCH == "arm64" {
		machine = 183
	}
	if binary.LittleEndian.Uint16(header[18:20]) != machine {
		return failure("linux_core_integrity_failed")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return failure("linux_core_integrity_failed")
	}
	hash := sha256.New()
	buffer := make([]byte, 1<<20)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := file.Read(buffer)
		total += int64(count)
		if total > target.Size {
			return failure("linux_core_integrity_failed")
		}
		_, _ = hash.Write(buffer[:count])
		if err == io.EOF {
			break
		}
		if err != nil {
			return failure("linux_core_integrity_failed")
		}
	}
	if total != target.Size || hex.EncodeToString(hash.Sum(nil)) != target.SHA256 {
		return failure("linux_core_integrity_failed")
	}
	return nil
}
