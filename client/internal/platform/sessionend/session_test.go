package sessionend

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepeatedShutdownWaitsForOneCleanup(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	r := newRequest(func(context.Context) { calls.Add(1); close(entered); <-release })
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); r.finish() }()
	}
	<-entered
	select {
	case <-r.done:
		t.Fatal("shutdown replied before cleanup")
	default:
	}
	close(release)
	workers.Wait()
	if calls.Load() != 1 {
		t.Fatal("cleanup was repeated")
	}
}

func TestHungCleanupCannotBlockSystemShutdown(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	r := newRequest(func(context.Context) { <-release })
	r.limit = 20 * time.Millisecond
	finished := make(chan struct{})
	go func() { r.finish(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown was blocked")
	}
}
