//go:build linux

package files

import "os/exec"

func openDirectory(path string) error {
	command := exec.Command("xdg-open", path)
	if err := command.Start(); err != nil {
		return err
	}
	// File managers may retain the launcher for the life of the window. Reap
	// asynchronously without blocking the binding or accumulating zombies.
	go func() { _ = command.Wait() }()
	return nil
}
