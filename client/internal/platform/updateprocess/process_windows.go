package updateprocess

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configure(command *exec.Cmd, hideWindow bool) {
	// CREATE_NO_WINDOW suppresses consoles; HideWindow additionally overrides
	// the GUI's first ShowWindow call with SW_HIDE and is only for the worker.
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: hideWindow, CreationFlags: 0x08000000}
}
func VerifyBundle(context.Context, string) error { return nil }

// Process teardown and antivirus readers can release an image shortly after
// the parent's pipe closes. Retry only Windows sharing/access failures.
func Rename(from, to string) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.Rename(from, to)
		if err == nil || time.Now().After(deadline) || (!errors.Is(err, syscall.Errno(5)) && !errors.Is(err, syscall.Errno(32)) && !errors.Is(err, syscall.Errno(33))) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}
