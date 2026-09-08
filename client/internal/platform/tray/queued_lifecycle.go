package tray

import "sync"

// queuedLifecycle serializes submissions to a native event loop. Cancelling
// before startup releases callbacks without enqueueing a future tray icon.
// The enqueue functions must not wait for native callbacks.
func queuedLifecycle(enqueueStart, enqueueStop, onCancelled func()) (func(), func()) {
	var mu sync.Mutex
	started, stopped := false, false
	start := func() {
		mu.Lock()
		defer mu.Unlock()
		if started || stopped {
			return
		}
		started = true
		enqueueStart()
	}
	stop := func() {
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		stopped = true
		if started {
			enqueueStop()
			mu.Unlock()
			return
		}
		mu.Unlock()
		onCancelled()
	}
	return start, stop
}
