//go:build linux

package runtime

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"jeemi/internal/platform/processguard"
)

func configureCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}
}

func retainProcessThread() func() {
	runtime.LockOSThread()
	return runtime.UnlockOSThread
}

func attachProcess(command *exec.Cmd) (func(), error) {
	if processguard.HasFileCapabilities(command.Path) {
		return processguard.Attach(command.Process)
	}
	return func() {}, nil
}

func terminateProcess(process *os.Process) error {
	return signalProcessGroup(process, syscall.SIGTERM)
}

func forceTerminateProcess(process *os.Process) error {
	return signalProcessGroup(process, syscall.SIGKILL)
}

func signalProcessGroup(process *os.Process, signal syscall.Signal) error {
	if err := syscall.Kill(-process.Pid, signal); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}

func errorsProcessDone(err error) bool { return errors.Is(err, os.ErrProcessDone) }
