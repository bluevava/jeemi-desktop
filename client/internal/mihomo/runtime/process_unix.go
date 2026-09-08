//go:build !windows && !linux

package runtime

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcess(*exec.Cmd) (func(), error) { return func() {}, nil }

func terminateProcess(process *os.Process) error {
	if err := syscall.Kill(-process.Pid, syscall.SIGTERM); err != nil {
		return process.Signal(os.Interrupt)
	}
	return nil
}

func errorsProcessDone(err error) bool { return errors.Is(err, os.ErrProcessDone) }
