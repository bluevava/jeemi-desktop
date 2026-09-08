package tray

import (
	"reflect"
	"sync"
	"testing"
)

func TestQueuedLifecycleCancelsBeforeStartWithoutLeavingNativeWork(t *testing.T) {
	var events []string
	start, stop := queuedLifecycle(
		func() { events = append(events, "start") },
		func() { events = append(events, "stop") },
		func() { events = append(events, "cancel") },
	)
	stop()
	start()
	stop()
	if !reflect.DeepEqual(events, []string{"cancel"}) {
		t.Fatalf("unexpected native work after cancellation: %v", events)
	}
}

func TestQueuedLifecycleSubmitsStartAndStopExactlyOnceInOrder(t *testing.T) {
	var events []string
	start, stop := queuedLifecycle(
		func() { events = append(events, "start") },
		func() { events = append(events, "stop") },
		func() { events = append(events, "cancel") },
	)
	start()
	start()
	stop()
	stop()
	start()
	if !reflect.DeepEqual(events, []string{"start", "stop"}) {
		t.Fatalf("unexpected native submission order: %v", events)
	}
}

func TestQueuedLifecycleConcurrentStopCannotOvertakeStart(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		var events []string
		start, stop := queuedLifecycle(
			func() { events = append(events, "start") },
			func() { events = append(events, "stop") },
			func() { events = append(events, "cancel") },
		)
		var workers sync.WaitGroup
		workers.Add(2)
		go func() { defer workers.Done(); start() }()
		go func() { defer workers.Done(); stop() }()
		workers.Wait()
		if !reflect.DeepEqual(events, []string{"cancel"}) && !reflect.DeepEqual(events, []string{"start", "stop"}) {
			t.Fatalf("stop overtook native startup: %v", events)
		}
	}
}
