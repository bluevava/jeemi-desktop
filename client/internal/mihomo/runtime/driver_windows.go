//go:build windows

package runtime

import (
	"context"
	"jeemi/internal/platform/winauth"
	"path/filepath"
	"sync"
	"time"
)

type windowsServiceDriver struct {
	commandDriver
	mu   sync.Mutex
	home string
}

func platformDriver() Driver { return &windowsServiceDriver{} }
func (d *windowsServiceDriver) Start(executable, home, configuration string) (Process, error) {
	target, err := winauth.TargetForExecutable(executable)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := winauth.SnapshotCall(ctx, winauth.Request{Operation: "start", Target: &target}, home, configuration, true)
	if err != nil {
		cleanup, release := context.WithTimeout(context.Background(), 5*time.Second)
		defer release()
		_, _ = winauth.Call(cleanup, winauth.Request{Operation: "stop"})
		return nil, err
	}
	d.mu.Lock()
	d.home = home
	d.mu.Unlock()
	p := &windowsServiceProcess{pid: reply.PID, done: make(chan struct{})}
	go p.watch()
	return p, nil
}
func (d *windowsServiceDriver) StageConfiguration(ctx context.Context, path string) (string, error) {
	d.mu.Lock()
	home := d.home
	d.mu.Unlock()
	if home == "" {
		home = filepath.Dir(filepath.Dir(filepath.Dir(path)))
	}
	reply, err := winauth.SnapshotCall(ctx, winauth.Request{Operation: "stage"}, home, path, false)
	return reply.Configuration, err
}

type windowsServiceProcess struct {
	pid      int
	done     chan struct{}
	mu       sync.Mutex
	err      error
	stopping bool
}

func (p *windowsServiceProcess) PID() int              { return p.pid }
func (p *windowsServiceProcess) Done() <-chan struct{} { return p.done }
func (p *windowsServiceProcess) ExitError() error      { p.mu.Lock(); defer p.mu.Unlock(); return p.err }
func (p *windowsServiceProcess) IsRunning(ctx context.Context) (bool, error) {
	state, err := winauth.Call(ctx, winauth.Request{Operation: "state"})
	return err == nil && state.Running && state.PID == p.pid, err
}
func (p *windowsServiceProcess) watch() {
	defer close(p.done)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		state, err := winauth.Call(ctx, winauth.Request{Operation: "state"})
		cancel()
		if err != nil || !state.Running || state.PID != p.pid {
			p.mu.Lock()
			if !p.stopping {
				p.err = err
				if p.err == nil {
					p.err = coreStoppedError()
				}
			}
			p.mu.Unlock()
			return
		}
		time.Sleep(time.Second)
	}
}
func coreStoppedError() error { return &serviceCoreError{} }

type serviceCoreError struct{}

func (*serviceCoreError) Error() string { return "authorization_core_failed" }
func (p *windowsServiceProcess) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopping = true
	p.mu.Unlock()
	if _, err := winauth.Call(ctx, winauth.Request{Operation: "stop"}); err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
