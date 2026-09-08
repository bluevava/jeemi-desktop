//go:build windows

package winauth

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
)

func managedCorePath(target Target) bool {
	path := filepath.Clean(target.ExecutablePath)
	volume := filepath.VolumeName(path)
	if !filepath.IsAbs(path) || len(volume) != 2 || volume[1] != ':' || strings.ContainsAny(path[2:], ":") {
		return false
	}
	for _, part := range []string{"mihomo.exe", "windows-" + runtime.GOARCH, target.Version, "mihomo", "core", "jeemi_data"} {
		if !strings.EqualFold(filepath.Base(path), part) {
			return false
		}
		path = filepath.Dir(path)
	}
	return true
}

// OpenInstalledCore verifies the selected core in place. The caller must retain
// the file and parent directory locks until the process has exited.
func OpenInstalledCore(ctx context.Context, target Target) (*LockedCore, error) {
	if !target.valid() || !managedCorePath(target) {
		return nil, failure("authorization_core_unverified")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Clean(target.ExecutablePath)
	core, err := lockCoreDirectories(ctx, path)
	if err != nil {
		return nil, err
	}
	valid := false
	defer func() {
		if !valid {
			core.Close()
		}
	}()
	file, err := lockFile(path)
	if err != nil {
		return nil, failure("authorization_core_unverified")
	}
	core.binary = file
	var attributes windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &attributes) != nil || attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, failure("authorization_core_unverified")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != target.Size {
		return nil, failure("authorization_core_changed")
	}
	if err = VerifyCore(ctx, file, target); err != nil {
		return nil, err
	}
	valid = true
	return core, nil
}
