package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"jeemi/internal/platform/coreauth"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

type Process interface {
	PID() int
	Done() <-chan struct{}
	ExitError() error
	Stop(context.Context) error
}

type Driver interface {
	Validate(context.Context, string, string, string) error
	Start(string, string, string) (Process, error)
}

type commandDriver struct{}

func newCommandDriver() Driver { return platformDriver() }

func (commandDriver) Preflight(executable string, preferences runtimeconfig.Preferences, runningPID int) error {
	if err := coreauth.Check(executable, runningPID); err != nil {
		return err
	}
	if preferences.ProxyMode == runtimeconfig.ProxyModeTUN {
		return requirements.CheckTUN(executable, runningPID)
	}
	return nil
}

// ValidateConfigurationAtHome validates one managed configuration in an
// isolated mihomo home. GEO asset management uses this narrow entry point so
// candidate databases are exercised by the selected official core without
// replacing files used by a running process.
func ValidateConfigurationAtHome(ctx context.Context, executablePath, homeDirectory, configurationPath string) error {
	return commandDriver{}.Validate(ctx, executablePath, homeDirectory, configurationPath)
}

func (commandDriver) Validate(ctx context.Context, executablePath, homeDirectory, configurationPath string) error {
	if err := validateExecutable(executablePath); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, executablePath, "-t", "-d", homeDirectory, "-f", configurationPath)
	command.Dir = homeDirectory
	configureCommand(command)
	output := newBoundedBuffer(64 << 10)
	command.Stdout = output
	command.Stderr = output
	releaseThread := retainProcessThread()
	defer releaseThread()
	command.WaitDelay = 2 * time.Second
	if err := command.Start(); err != nil {
		return fmt.Errorf("start mihomo validation: %w", err)
	}
	release, err := attachProcess(command)
	if err != nil {
		_ = forceTerminateProcess(command.Process)
		_ = command.Wait()
		return fmt.Errorf("attach mihomo validation lifecycle: %w", err)
	}
	defer release()
	if err := command.Wait(); err != nil {
		detail := sanitizeDiagnostic(output.String())
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("mihomo configuration validation failed: %s", detail)
	}
	return nil
}

func (commandDriver) Start(executablePath, homeDirectory, configurationPath string) (Process, error) {
	if err := validateExecutable(executablePath); err != nil {
		return nil, err
	}
	command := exec.Command(executablePath, "-d", homeDirectory, "-f", configurationPath)
	command.Dir = homeDirectory
	configureCommand(command)
	output := newBoundedBuffer(128 << 10)
	command.Stdout = output
	command.Stderr = output
	command.WaitDelay = 2 * time.Second
	type startResult struct {
		process *commandProcess
		err     error
	}
	started := make(chan startResult, 1)
	go func() {
		// Linux Pdeathsig follows the creating OS thread. Keep that thread alive
		// until Wait finishes, so Go retiring a thread cannot terminate mihomo.
		releaseThread := retainProcessThread()
		defer releaseThread()
		if err := command.Start(); err != nil {
			started <- startResult{err: fmt.Errorf("start mihomo process: %w", err)}
			return
		}
		release, err := attachProcess(command)
		if err != nil {
			_ = forceTerminateProcess(command.Process)
			_ = command.Wait()
			started <- startResult{err: fmt.Errorf("attach mihomo process lifecycle: %w", err)}
			return
		}
		process := &commandProcess{command: command, output: output, done: make(chan struct{}), release: release}
		started <- startResult{process: process}
		process.wait()
	}()
	result := <-started
	if result.err != nil {
		return nil, result.err
	}
	return result.process, result.err
}

func validateExecutable(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("mihomo executable path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("selected mihomo executable is unavailable")
	}
	return nil
}

type commandProcess struct {
	command *exec.Cmd
	output  *boundedBuffer
	done    chan struct{}
	release func()

	mu      sync.RWMutex
	exitErr error
}

func (p *commandProcess) PID() int              { return p.command.Process.Pid }
func (p *commandProcess) Done() <-chan struct{} { return p.done }
func (p *commandProcess) ExitError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitErr
}

func (p *commandProcess) wait() {
	err := p.command.Wait()
	if err != nil {
		if detail := sanitizeDiagnostic(p.output.String()); detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
	}
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
	if p.release != nil {
		p.release()
	}
	close(p.done)
}

func (p *commandProcess) Stop(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := terminateProcess(p.command.Process); err != nil && !errorsProcessDone(err) {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		if err := forceTerminateProcess(p.command.Process); err != nil && !errorsProcessDone(err) {
			return err
		}
		// The caller's deadline has expired; allow a separate bounded interval
		// to reap the killed process instead of immediately racing that deadline.
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-p.done:
			return nil
		case <-timer.C:
			return ctx.Err()
		}
	}
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newBoundedBuffer(limit int) *boundedBuffer { return &boundedBuffer{limit: limit} }

func (b *boundedBuffer) Write(contents []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, contents...)
	if len(b.data) > b.limit {
		b.data = append([]byte{}, b.data[len(b.data)-b.limit:]...)
	}
	return len(contents), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.Clone(b.data))
}

var _ io.Writer = (*boundedBuffer)(nil)
