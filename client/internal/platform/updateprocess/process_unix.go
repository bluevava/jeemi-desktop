//go:build linux || darwin

package updateprocess

import (
	"os"
	"os/exec"
	"syscall"
)

func configure(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func Rename(from, to string) error { return os.Rename(from, to) }
