//go:build darwin

package runtime

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"jeemi/internal/platform/macnetwork"
	"jeemi/internal/runtimeconfig"
)

type macDriver struct {
	commandDriver
	mu     sync.Mutex
	device string
}

func platformDriver() Driver { return &macDriver{} }
func (d *macDriver) Start(executable, home, configuration string) (Process, error) {
	target, err := macnetwork.TargetForExecutable(executable)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(home, configuration)
	if err != nil {
		return nil, macnetwork.Failure("unsafe_path")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reply, err := macnetwork.Start(ctx, target, filepath.ToSlash(relative))
	if err != nil {
		return nil, err
	}
	process := &macProcess{pid: reply.PID, done: make(chan struct{})}
	go process.watch()
	return process, nil
}
func (d *macDriver) ActivateTUN(ctx context.Context, preferences runtimeconfig.Preferences) error {
	dns := ""
	if preferences.DNSEnabled {
		dns = preferences.DNSListen
	}
	reply, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "tun", DNS: dns})
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.device = reply.Device
	d.mu.Unlock()
	return nil
}
func (d *macDriver) TUNDevice() string { d.mu.Lock(); defer d.mu.Unlock(); return d.device }
func (d *macDriver) DeactivateNetwork(ctx context.Context) error {
	_, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "restore"})
	if err == nil {
		d.mu.Lock()
		d.device = ""
		d.mu.Unlock()
	}
	return err
}

type macProcess struct {
	pid      int
	done     chan struct{}
	mu       sync.Mutex
	err      error
	stopping bool
}

func (p *macProcess) PID() int              { return p.pid }
func (p *macProcess) Done() <-chan struct{} { return p.done }
func (p *macProcess) ExitError() error      { p.mu.Lock(); defer p.mu.Unlock(); return p.err }
func (p *macProcess) IsRunning(ctx context.Context) (bool, error) {
	state, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "state"})
	return err == nil && state.Running && state.PID == p.pid, err
}
func (p *macProcess) watch() {
	defer close(p.done)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		state, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "state"})
		cancel()
		if err != nil || !state.Running || state.PID != p.pid {
			p.mu.Lock()
			if !p.stopping {
				p.err = macnetwork.Failure("core_failed")
			}
			p.mu.Unlock()
			return
		}
		time.Sleep(time.Second)
	}
}
func (p *macProcess) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopping = true
	p.mu.Unlock()
	_, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "stop"})
	if err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *macDriver) StageConfiguration(ctx context.Context, path string) (string, error) {
	// The GUI derives the generation path. The helper independently restricts it
	// to a fixed generation filename and an unprivileged snapshot operation.
	directory := filepath.Base(filepath.Dir(path))
	relative := "generations/" + directory + "/" + filepath.Base(path)
	response, err := macnetwork.Call(ctx, macnetwork.Request{Operation: "stage", Configuration: relative})
	return response.Configuration, err
}
