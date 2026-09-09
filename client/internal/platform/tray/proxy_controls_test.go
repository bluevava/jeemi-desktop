package tray

import (
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func assertProxyMenu(t *testing.T, controller *Controller, want [3]bool) {
	t.Helper()
	controller.menuUpdateMu.Lock()
	defer controller.menuUpdateMu.Unlock()
	for index, item := range controller.proxyItems {
		if got := item.(*fakeMenuItem).enabled; got != want[index] {
			t.Fatalf("proxy menu %d enabled=%v, want %v", index, got, want[index])
		}
	}
}

func TestProxyControlsFollowRuntimeAndRecheckBeforeDispatch(t *testing.T) {
	var state atomic.Value
	state.Store(ProxyState{})
	var calls [3]int
	controller := newController([]byte{1}, nil, &fakeBackend{})
	controller.Start(nil, nil, nil, ProxyControls{
		State:   func() ProxyState { return state.Load().(ProxyState) },
		Start:   func() { calls[0]++; state.Store(ProxyState{Running: true}) },
		Stop:    func() { calls[1]++; state.Store(ProxyState{}) },
		Restart: func() { calls[2]++ },
	})
	defer controller.Stop()
	assertProxyMenu(t, controller, [3]bool{true, false, false})
	controller.runProxyAction(1)
	controller.runProxyAction(2)
	if calls != [3]int{} {
		t.Fatal("stopped proxy accepted stop/restart")
	}
	controller.runProxyAction(0)
	assertProxyMenu(t, controller, [3]bool{false, true, true})
	controller.runProxyAction(2)
	assertProxyMenu(t, controller, [3]bool{false, true, true})
	controller.runProxyAction(1)
	assertProxyMenu(t, controller, [3]bool{true, false, false})
	if calls != [3]int{1, 1, 1} {
		t.Fatalf("actions not routed correctly: %v", calls)
	}

	// A failed start keeps the actual state stopped and must remain retryable.
	controller.mu.Lock()
	controller.proxy.Start = func() { calls[0]++ }
	controller.mu.Unlock()
	controller.runProxyAction(0)
	assertProxyMenu(t, controller, [3]bool{true, false, false})

	state.Store(ProxyState{Running: true})
	controller.refreshProxy()
	assertProxyMenu(t, controller, [3]bool{false, true, true})
	// Simulate an exit or authorization started from Home before the next tick.
	state.Store(ProxyState{Busy: true})
	controller.runProxyAction(2)
	controller.refreshProxy()
	assertProxyMenu(t, controller, [3]bool{})
	if calls[2] != 1 {
		t.Fatal("stale enabled item restarted an unavailable proxy")
	}
	controller.Stop()
	state.Store(ProxyState{})
	controller.runProxyAction(0)
	if calls[0] != 2 {
		t.Fatal("action ran after tray shutdown")
	}
}

func TestProxyMenuDispatchDoesNotBlockNativeLoopOrAcceptDuplicateActions(t *testing.T) {
	native := &fakeBackend{}
	controller := newController([]byte{1}, nil, native)
	entered, release := make(chan struct{}), make(chan struct{})
	var count atomic.Int32
	controller.Start(nil, nil, nil, ProxyControls{
		State: func() ProxyState { return ProxyState{} },
		Start: func() { count.Add(1); close(entered); <-release },
	})
	defer controller.Stop()
	defer close(release)
	returned := make(chan struct{})
	go func() { native.items[2].click(); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("native callback was blocked")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("proxy action was not dispatched")
	}
	assertProxyMenu(t, controller, [3]bool{})
	controller.runProxyAction(0)
	if count.Load() != 1 {
		t.Fatal("duplicate action was dispatched")
	}
	// Closing the tray must not wait for a long-running proxy action.
	controller.Stop()
}

func TestProxyMenuRefreshesWithoutRendererRequests(t *testing.T) {
	var running atomic.Bool
	controller := newController([]byte{1}, nil, &fakeBackend{})
	controller.Start(nil, nil, nil, ProxyControls{
		State: func() ProxyState { return ProxyState{Running: running.Load()} },
		Start: func() {}, Stop: func() {}, Restart: func() {},
	})
	defer controller.Stop()
	running.Store(true)
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		controller.menuUpdateMu.Lock()
		ready := controller.proxyItems[1].(*fakeMenuItem).enabled
		controller.menuUpdateMu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatal("background runtime change did not update the tray")
		case <-tick.C:
		}
	}
	assertProxyMenu(t, controller, [3]bool{false, true, true})
}

func TestSharedTrayResourcesContainAllControlAndFailureLabels(t *testing.T) {
	contents, err := os.ReadFile("../../../frontend/src/i18n/tray-labels.json")
	if err != nil {
		t.Fatal(err)
	}
	var labels map[string]Labels
	if err := json.Unmarshal(contents, &labels); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{LanguageChinese, LanguageEnglish} {
		if err := validateLabels(language, labels[language]); err != nil {
			t.Fatal(err)
		}
	}
}
