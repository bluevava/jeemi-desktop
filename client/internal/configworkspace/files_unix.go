//go:build !windows

package configworkspace

import (
	"io/fs"
	"os"
)

func unsafeFile(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular())
}
func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
