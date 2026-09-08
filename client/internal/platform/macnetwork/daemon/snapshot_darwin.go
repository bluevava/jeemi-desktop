//go:build darwin

package daemon

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"jeemi/internal/platform/macnetwork"
)

func SnapshotCommand(configuration string, initial bool) error {
	if os.Geteuid() == 0 || os.Getuid() == 0 {
		return macnetwork.Failure("unsafe_path")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return WriteSnapshot(filepath.Join(home, "Library/Application Support/jeemi_data/runtime/mihomo"), configuration, initial, os.Stdout)
}

func snapshotAsUser(uid uint32, configuration, destination string, initial bool, digests map[string]string) error {
	if uid < 500 || !configurationName(configuration) {
		return macnetwork.Failure("unsafe_path")
	}
	account, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return macnetwork.Failure("unsafe_path")
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		return macnetwork.Failure("unsafe_path")
	}
	executable, err := os.Executable()
	if err != nil {
		return macnetwork.Failure("helper_failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	mode := "update"
	if initial {
		mode = "initial"
	}
	child := exec.CommandContext(ctx, executable, "--snapshot", configuration, mode)
	child.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + account.HomeDir, "LANG=en_US.UTF-8"}
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: uint32(gid), Groups: []uint32{}}}
	child.Stderr = io.Discard
	child.WaitDelay = time.Second
	output, err := child.StdoutPipe()
	if err != nil {
		return macnetwork.Failure("helper_failed")
	}
	if err = child.Start(); err != nil {
		return macnetwork.Failure("helper_failed")
	}
	userHome := filepath.Join(account.HomeDir, "Library/Application Support/jeemi_data/runtime/mihomo")
	err = receiveSnapshot(destination, userHome, output, digests)
	if err != nil {
		_ = child.Process.Kill()
	}
	waitErr := child.Wait()
	if err != nil || waitErr != nil {
		return macnetwork.Failure("unsafe_path")
	}
	return nil
}

func removeWorkspace(directory string) {
	// Directory is derived here, never accepted from IPC or an archive.
	if filepath.Dir(directory) == rootDirectory && strings.HasPrefix(filepath.Base(directory), "session-") {
		_ = os.RemoveAll(directory)
	}
}
