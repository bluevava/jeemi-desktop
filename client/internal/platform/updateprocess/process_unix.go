//go:build linux || darwin

package updateprocess

import (
	"os"
	"os/exec"
	"syscall"
)

func configure(command *exec.Cmd, _ bool) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func Rename(from, to string) error { return os.Rename(from, to) }
