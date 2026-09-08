package appupdate

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"jeemi/internal/platform/paths"
	"jeemi/internal/platform/updateprocess"
)

const workerArgument = "--jeemi-apply-update"

type workerHandle struct {
	pipe    io.WriteCloser
	command *exec.Cmd
}

func (w *workerHandle) abort() {
	_ = w.pipe.Close()
	done := make(chan struct{})
	go func() { _ = w.command.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = w.command.Process.Kill()
		<-done
	}
}
func (w *workerHandle) commit() error {
	if _, err := io.WriteString(w.pipe, "commit\n"); err != nil {
		return ErrRestart
	}
	// Keep stdin open until process exit. The worker cannot replace files while
	// this GUI (and its single-instance lock) is still alive.
	return nil
}

func launchWorker(ctx context.Context, jobRoot string, job installJob) (*workerHandle, error) {
	data, err := json.Marshal(job)
	if err != nil || os.WriteFile(filepath.Join(jobRoot, "job.json"), data, 0600) != nil {
		return nil, ErrRestart
	}
	runner := filepath.Join(jobRoot, "runner")
	if runtime.GOOS == "windows" {
		runner += ".exe"
	}
	if err = copyTree(job.Executable, runner); err != nil {
		return nil, ErrRestart
	}
	command := exec.Command(runner, workerArgument, job.ID)
	input, err := command.StdinPipe()
	if err != nil {
		return nil, ErrRestart
	}
	output, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, ErrRestart
	}
	if err = updateprocess.Start(command); err != nil {
		_ = input.Close()
		_ = output.Close()
		return nil, ErrRestart
	}
	handle := &workerHandle{input, command}
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(io.LimitReader(output, 256)).ReadString('\n')
		ready <- strings.TrimSpace(line)
	}()
	select {
	case line := <-ready:
		if line == "ready" {
			return handle, nil
		}
		handle.abort()
		if line == ErrLocation.Error() {
			return nil, ErrLocation
		}
		if line == ErrPackage.Error() {
			return nil, ErrPackage
		}
		return nil, ErrRestart
	case <-ctx.Done():
		handle.abort()
		return nil, ErrCancelled
	case <-time.After(60 * time.Second):
		handle.abort()
		return nil, ErrRestart
	}
}

// RunIfRequested is dispatched before desktop assembly, so this ordinary-user
// updater creates no WebView, tray, authorization service or second instance.
func RunIfRequested() (bool, error) {
	if len(os.Args) < 2 || os.Args[1] != workerArgument {
		return false, nil
	}
	err := runWorker()
	if err != nil {
		_, _ = io.WriteString(os.Stdout, err.Error()+"\n")
	}
	return true, err
}

func runWorker() error {
	if len(os.Args) != 3 || !idPattern.MatchString(os.Args[2]) {
		return ErrRestart
	}
	dataRoot, err := paths.DataDirectory()
	if err != nil {
		return ErrRestart
	}
	root := filepath.Join(dataRoot, "updates", "jeemi")
	jobRoot, err := filepath.EvalSymlinks(filepath.Join(root, os.Args[2]))
	if err != nil {
		return ErrRestart
	}
	self, err := os.Executable()
	if err != nil {
		return ErrRestart
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil || filepath.Dir(self) != jobRoot {
		return ErrRestart
	}
	var job installJob
	data, err := readBounded(filepath.Join(jobRoot, "job.json"), 16384)
	if err != nil || json.Unmarshal(data, &job) != nil || job.ID != os.Args[2] {
		return ErrRestart
	}
	t, err := targetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	replacement, err := prepareReplacement(ctx, jobRoot, job, t)
	if err != nil {
		return err
	}
	retainBackup := false
	defer func() {
		if !retainBackup {
			replacement.cleanup()
		}
	}()
	_, _ = io.WriteString(os.Stdout, "ready\n")
	gate := make(chan bool, 1)
	go func() { gate <- committedExit(os.Stdin) }()
	select {
	case committed := <-gate:
		if !committed {
			return ErrCancelled
		}
	case <-time.After(90 * time.Second):
		return ErrRestart
	}
	// Check once more after the parent exits, before modifying the installation.
	digest, _, err := hashFile(job.Executable)
	if err != nil || digest != job.CurrentSHA256 {
		return ErrLocation
	}
	err = completeUpdate(replacement, func() error { return restartAndWait(jobRoot, job) })
	if err == ErrRollback {
		retainBackup = true
	}
	receipt := State{Phase: "succeeded", Version: job.Version, JobID: job.ID}
	if err != nil {
		receipt.Phase = "failed"
		receipt.Error = err.Error()
	}
	writeReceipt(root, receipt)
	if err != nil && err != ErrRollback {
		// Old files have been restored. Launch them without the update argument.
		command := exec.Command(job.Executable)
		command.Dir = filepath.Dir(job.Executable)
		_ = updateprocess.Start(command)
	}
	// Retain a small receipt and the worker (Windows holds its image open).
	// The downloaded archive and extracted duplicate are no longer needed.
	_ = os.RemoveAll(filepath.Join(jobRoot, "extracted"))
	_ = os.Remove(filepath.Join(jobRoot, "archive"+t.extension))
	return err
}

func committedExit(input io.Reader) bool {
	reader := bufio.NewReader(io.LimitReader(input, 32))
	line, err := reader.ReadString('\n')
	if err != nil || line != "commit\n" {
		return false
	}
	_, err = reader.ReadByte()
	return err == io.EOF
}

func completeUpdate(replacement *replacement, restart func() error) error {
	if err := replacement.apply(); err != nil {
		return err
	}
	err := restart()
	// If a child could not be stopped, retain backups instead of overwriting
	// files that may still be executing.
	if err != nil && err != ErrRollback && replacement.rollback() != nil {
		return ErrRollback
	}
	return err
}

func restartAndWait(jobRoot string, job installJob) error {
	_ = os.Remove(filepath.Join(jobRoot, "started"))
	command := exec.Command(job.Executable, "--jeemi-update-restarted", job.ID)
	command.Dir = filepath.Dir(job.Executable)
	if updateprocess.Start(command) != nil {
		return ErrRestart
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(60 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ticker.C:
			if data, err := readBounded(filepath.Join(jobRoot, "started"), 256); err == nil && strings.TrimPrefix(string(data), "v") == strings.TrimPrefix(job.Version, "v") {
				return nil
			}
		case <-done:
			return ErrRestart
		case <-timeout.C:
			_ = command.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				return ErrRollback
			}
			return ErrRestart
		}
	}
}

func writeReceipt(root string, state State) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	temp := filepath.Join(root, "result.json.tmp")
	if os.WriteFile(temp, data, 0600) == nil {
		_ = os.Rename(temp, filepath.Join(root, "result.json"))
	}
}
