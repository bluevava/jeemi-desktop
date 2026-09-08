//go:build linux

package processguard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) {
	if handled, err := RunIfRequested(); handled {
		if err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "--test-guard-parent" {
		// Simulates the GUI process, with an ordinary inert child whose missing
		// Pdeathsig models Linux clearing that signal on a capability-bearing exec.
		child, ready := startInertChild()
		if child == nil || !ready {
			os.Exit(3)
		}
		if _, err := Attach(child.Process); err != nil {
			_ = child.Process.Kill()
			os.Exit(4)
		}
		fmt.Println(child.Process.Pid)
		time.Sleep(time.Hour)
		os.Exit(5)
	}
	os.Exit(m.Run())
}

func startInertChild() (*exec.Cmd, bool) {
	command := exec.Command("/bin/sh", "-c", "trap '' TERM; echo ready; exec sleep 60")
	output, err := command.StdoutPipe()
	if err != nil || command.Start() != nil {
		return nil, false
	}
	line, err := bufio.NewReader(output).ReadString('\n')
	return command, err == nil && strings.TrimSpace(line) == "ready"
}

func TestGuardTerminatesCoreWhenGUIIsKilled(t *testing.T) {
	executable, _ := os.Executable()
	parent := exec.Command(executable, "--test-guard-parent")
	stdout, err := parent.StdoutPipe()
	if err != nil || parent.Start() != nil {
		t.Fatal("start fake GUI")
	}
	t.Cleanup(func() { _ = parent.Process.Kill(); _ = parent.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal("fake GUI failed to install guard")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0)
		_ = unix.Close(pidfd)
	})
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	fds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, _ = unix.Poll(fds, 100)
		if fds[0].Revents&unix.POLLIN != 0 {
			return
		}
	}
	t.Fatal("core survived GUI death despite lifetime guard")
}

func TestGuardReleaseIsIdempotentAfterCoreExit(t *testing.T) {
	command, ready := startInertChild()
	if command == nil || !ready {
		t.Fatal("start inert child")
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	release, err := Attach(command.Process)
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	release()
	release()
}

func TestGuardRejectsInvocationWithoutInheritedDescriptors(t *testing.T) {
	if err := run(-1, -2, -3); err == nil {
		t.Fatal("invalid lifetime descriptors accepted")
	}
}
