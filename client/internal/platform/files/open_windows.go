//go:build windows

package files

import "os/exec"

func openDirectory(path string) error {
	return exec.Command("explorer.exe", path).Start()
}
