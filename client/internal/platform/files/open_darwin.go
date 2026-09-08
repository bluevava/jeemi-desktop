//go:build darwin

package files

import "os/exec"

func openDirectory(path string) error {
	return exec.Command("open", path).Start()
}
