//go:build windows

package daemon

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// Core file I/O must not hold the network/lifecycle mutex. Shutdown/removal
// cancels and joins verification before the service stops or is removed.
type preparationGate struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	closing bool
}

func (g *preparationGate) begin() (context.Context, func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing || g.cancel != nil {
		return nil, nil, failure("authorization_busy")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	done := make(chan struct{})
	g.cancel, g.done = cancel, done
	finish := func() {
		cancel()
		g.mu.Lock()
		defer g.mu.Unlock()
		g.cancel, g.done = nil, nil
		close(done)
	}
	return ctx, finish, nil
}

func (g *preparationGate) close(ctx context.Context) error {
	g.mu.Lock()
	g.closing = true
	done := g.done
	if g.cancel != nil {
		g.cancel()
	}
	g.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *preparationGate) resume() {
	g.mu.Lock()
	g.closing = false
	g.mu.Unlock()
}

func (s *server) prepareCore(caller windows.Handle, target Target) error {
	select {
	case <-s.shutdown:
		return failure("authorization_repair_required")
	default:
	}
	ctx, finish, err := s.preparation.begin()
	if err != nil {
		return err
	}
	defer finish()
	// Both check and prepare verify the selected file in place. Start reopens
	// it and keeps the locks, so changes after this check cannot bypass hashing.
	binary, err := openCallerCore(ctx, caller, target)
	if err != nil {
		return err
	}
	return binary.Close()
}
