// Package sessionend handles confirmed OS logout/shutdown separately from a
// window close. Cleanup is bounded: Jeemi must never veto the user's shutdown.
package sessionend

import (
	"context"
	"sync"
	"time"
)

const cleanupTimeout = 4 * time.Second

type request struct {
	once    sync.Once
	done    chan struct{}
	cleanup func(context.Context)
	begin   func()
	limit   time.Duration
}

func newRequest(cleanup func(context.Context)) *request {
	return &request{done: make(chan struct{}), cleanup: cleanup, limit: cleanupTimeout}
}

func (r *request) finish() {
	r.once.Do(func() {
		if r.begin != nil {
			r.begin()
		}
		go func() {
			defer close(r.done)
			ctx, cancel := context.WithTimeout(context.Background(), r.limit)
			defer cancel()
			finished := make(chan struct{})
			go func() { defer close(finished); r.cleanup(ctx) }()
			select {
			case <-finished:
			case <-ctx.Done():
			}
		}()
	})
	<-r.done
}

// Start watches native session termination only in the running main instance.
// quit returns control to Wails where the platform does not terminate the app.
func Start(begin func(), cleanup func(context.Context), quit func()) (func(), error) {
	r := newRequest(cleanup)
	r.begin = begin
	return start(r, quit)
}
