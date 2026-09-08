//go:build linux

// Package processguard provides a short-lived, unprivileged lifetime guard for
// a capability-bearing mihomo child. Linux clears Pdeathsig on privileged exec.
package processguard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const internalArgument = "--jeemi-watch-mihomo"

// RunIfRequested is called before GUI/data initialization. The private mode has
// no path, PID, command, or privilege arguments: it only accepts inherited FDs.
func RunIfRequested() (bool, error) {
	if len(os.Args) != 2 || os.Args[1] != internalArgument {
		return false, nil
	}
	return true, run(3, 4, 5)
}

// Attach uses a pidfd so a recycled numeric PID can never be signalled. The core
// remains a direct child of Jeemi; this sibling does not launch it or act as a
// persistent service. Attach must finish before enabling proxy/TUN networking.
func Attach(process *os.Process) (func(), error) {
	pidfd, err := unix.PidfdOpen(process.Pid, 0)
	if err != nil {
		return nil, fmt.Errorf("Linux core lifetime protection requires pidfd support (Linux 5.3+): %w", err)
	}
	pidFile := os.NewFile(uintptr(pidfd), "mihomo-pidfd")
	defer pidFile.Close()
	controlRead, controlWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer controlRead.Close()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		controlWrite.Close()
		return nil, err
	}
	defer readyRead.Close()
	defer readyWrite.Close()
	executable, err := os.Executable()
	if err != nil {
		controlWrite.Close()
		return nil, err
	}
	command := exec.Command(executable, internalArgument)
	command.ExtraFiles = []*os.File{pidFile, controlRead, readyWrite}
	// No Pdeathsig: this guard must briefly outlive the GUI to clean its core.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		controlWrite.Close()
		return nil, fmt.Errorf("start Linux core lifetime guard: %w", err)
	}
	readyWrite.Close()
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		// An unexpectedly failed guard must not leave a privileged core running
		// without protection. os.Process retains an identity-safe handle on Linux.
		_ = process.Kill()
		close(done)
	}()
	if err := waitReady(int(readyRead.Fd())); err != nil {
		controlWrite.Close()
		_ = command.Process.Kill()
		<-done
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			controlWrite.Close()
			select {
			case <-done:
			case <-time.After(4 * time.Second):
				_ = command.Process.Kill()
			}
		})
	}, nil
}

func waitReady(fd int) error {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, err := unix.Poll(fds, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			break
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			var value [1]byte
			if count, err := unix.Read(fd, value[:]); err == nil && count == 1 && value[0] == 1 {
				return nil
			}
			break
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			break
		}
	}
	return fmt.Errorf("Linux core lifetime guard did not become ready")
}

func run(pidfd, control, ready int) error {
	// Reject a manual invocation without the exact descriptor types before any
	// signal. All subsequent signals target this pidfd, never a supplied PID.
	link, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", pidfd))
	if err != nil || link != "anon_inode:[pidfd]" {
		return fmt.Errorf("invalid core lifetime descriptor")
	}
	var controlStat, readyStat unix.Stat_t
	if unix.Fstat(control, &controlStat) != nil || controlStat.Mode&unix.S_IFMT != unix.S_IFIFO ||
		unix.Fstat(ready, &readyStat) != nil || readyStat.Mode&unix.S_IFMT != unix.S_IFIFO {
		return fmt.Errorf("invalid core lifetime channel")
	}
	defer unix.Close(pidfd)
	defer unix.Close(control)
	defer unix.Close(ready)
	if _, err := unix.Write(ready, []byte{1}); err != nil {
		return terminate(pidfd)
	}
	fds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}, {Fd: int32(control), Events: unix.POLLIN}}
	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return terminate(pidfd)
		}
		if fds[0].Revents != 0 {
			return nil
		} // core exited; no GUI notification needed
		if fds[1].Revents != 0 {
			return terminate(pidfd)
		} // EOF, invalid input, or parent died
	}
}

func terminate(pidfd int) error {
	if err := unix.PidfdSendSignal(pidfd, unix.SIGTERM, nil, 0); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return nil
		}
		return fmt.Errorf("terminate unmanaged core: %w", err)
	}
	fds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := unix.Poll(fds, 100)
		if err == nil && fds[0].Revents != 0 {
			return nil
		}
	}
	if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	return nil
}

func HasFileCapabilities(executable string) bool {
	// Any file capability can clear Pdeathsig, not just CAP_NET_ADMIN.
	count, err := unix.Getxattr(executable, "security.capability", nil)
	return err == nil && count > 0
}
