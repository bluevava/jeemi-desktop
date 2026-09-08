//go:build windows

package winauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"golang.org/x/sys/windows"
)

// LockedCore keeps the verified executable and its path stable without changing
// user directory permissions. Only OpenInstalledCore can create a usable value.
type LockedCore struct {
	binary      *os.File
	directories []*os.File
	once        sync.Once
	closeErr    error
}

func (c *LockedCore) Name() string { return c.binary.Name() }

func (c *LockedCore) Close() error {
	c.once.Do(func() {
		if c.binary != nil {
			c.closeErr = c.binary.Close()
		}
		for _, directory := range slices.Backward(c.directories) {
			c.closeErr = errors.Join(c.closeErr, directory.Close())
		}
	})
	return c.closeErr
}

func lockCoreDirectories(ctx context.Context, executable string) (*LockedCore, error) {
	var paths []string
	for directory := filepath.Dir(executable); ; directory = filepath.Dir(directory) {
		paths = append(paths, directory)
		if filepath.Dir(directory) == directory {
			break
		}
	}
	core := &LockedCore{}
	for _, path := range slices.Backward(paths) {
		if err := ctx.Err(); err != nil {
			core.Close()
			return nil, err
		}
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			core.Close()
			return nil, failure("authorization_core_unverified")
		}
		// Walk from the volume root, refusing reparse points and holding every
		// ancestor against rename/delete before opening the next component.
		handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			core.Close()
			return nil, failure("authorization_core_unverified")
		}
		core.directories = append(core.directories, os.NewFile(uintptr(handle), path))
		var info windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(handle, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			core.Close()
			return nil, failure("authorization_core_unverified")
		}
	}
	return core, nil
}
