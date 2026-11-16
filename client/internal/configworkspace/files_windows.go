//go:build windows

package configworkspace

import (
	"io/fs"
	"syscall"
)

func unsafeFile(info fs.FileInfo) bool {
	if attrs, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return true
	}
	return info.Mode()&fs.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular())
}

// Windows publication uses same-volume directory rename; individual files have
// already been flushed. Directory Sync is not supported by the Windows API.
func syncDirectory(string) error { return nil }
