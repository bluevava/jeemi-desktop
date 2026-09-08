package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"jeemi/internal/runtimeconfig"
)

const manualGroup = "🌐 Main / Group"

// All platforms use this fake core: Windows/macOS lose the entire core cache
// at process start, Linux retains its group-name-only cache. No native helper,
// real proxy, TUN interface or external network is used.
type selectionCore struct {
	mu            sync.Mutex
	groups        map[string]proxySelectionState
	cache         map[string]string
	preserveCache bool
	storeSelected bool
	puts          []string
	failSelection bool
	failSnapshot  bool
	failReload    bool
}

func newSelectionCore(goos string) *selectionCore {
	return &selectionCore{
		groups: map[string]proxySelectionState{
			manualGroup: {Type: "Selector", All: []string{"Node A", "Node B", "Auto"}},
			"GLOBAL":    {Type: "Selector", All: []string{"DIRECT", "Node A", "Node B"}},
			"Auto":      {Type: "URLTest", All: []string{"Node A", "Node B"}},
			"Fallback":  {Type: "Fallback", All: []string{"Node A", "Node B"}},
			"Balance":   {Type: "LoadBalance", All: []string{"Node A", "Node B"}},
		},
		cache: map[string]string{}, preserveCache: goos == "linux", storeSelected: true,
	}
}

func (c *selectionCore) load(path string, start bool) error {
	defaults, err := selectorDefaults(path)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if start && !c.preserveCache {
		c.cache = map[string]string{}
	}
	for name, state := range c.groups {
		state.Now = ""
		if len(state.All) > 0 {
			state.Now = state.All[0]
			if state.Type != "Selector" {
				state.Now = state.All[len(state.All)-1]
			}
		}
		if slices.Contains(state.All, defaults[name]) {
			state.Now = defaults[name]
		}
		if c.storeSelected && slices.Contains(state.All, c.cache[name]) {
			state.Now = c.cache[name]
		}
		c.groups[name] = state
	}
	return nil
}

func (c *selectionCore) RoundTrip(request *http.Request) (*http.Response, error) {
	status := http.StatusNoContent
	body := ""
	if request.Method == http.MethodPut && request.URL.Path == "/configs" {
		var input struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			return nil, err
		}
		c.mu.Lock()
		fail := c.failReload
		c.failReload = false
		c.mu.Unlock()
		if fail {
			status = http.StatusBadRequest
		} else if err := c.load(input.Path, false); err != nil {
			return nil, err
		}
	} else {
		c.mu.Lock()
		defer c.mu.Unlock()
		switch {
		case request.URL.Path == "/version":
			status, body = http.StatusOK, `{"version":"v1.19.30"}`
		case request.Method == http.MethodGet && request.URL.Path == "/configs":
			status, body = http.StatusOK, `{"mode":"rule"}`
		case request.Method == http.MethodGet && request.URL.Path == "/proxies":
			status = http.StatusOK
			if c.failSnapshot {
				status = http.StatusServiceUnavailable
			} else {
				data, _ := json.Marshal(map[string]any{"proxies": c.groups})
				body = string(data)
			}
		case request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/proxies/"):
			var input struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				return nil, err
			}
			group := strings.TrimPrefix(request.URL.Path, "/proxies/")
			state, found := c.groups[group]
			if c.failSelection || !found || !slices.Contains(state.All, input.Name) {
				status = http.StatusBadRequest
			} else {
				state.Now = input.Name
				c.groups[group] = state
				if c.storeSelected {
					c.cache[group] = input.Name
				}
				c.puts = append(c.puts, group)
			}
		}
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

func (c *selectionCore) now(group string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.groups[group].Now
}

type selectionDriver struct {
	fakeRuntimeDriver
	core *selectionCore
}

func (d *selectionDriver) Start(binary, home, configuration string) (Process, error) {
	if err := d.core.load(configuration, true); err != nil {
		return nil, err
	}
	return d.fakeRuntimeDriver.Start(binary, home, configuration)
}

func selectionManager(t *testing.T, root, goos string, core *selectionCore) (*Manager, *selectionDriver, *fakeSystemProxy) {
	t.Helper()
	driver, proxy := &selectionDriver{core: core}, &fakeSystemProxy{}
	m, err := NewManager(Options{DataDirectory: root, Driver: driver, SystemProxy: proxy,
		HTTPClient: &http.Client{Transport: core}, GOOS: goos, RestartDelays: []time.Duration{time.Millisecond},
		ObserveTUN: func(context.Context, string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m, driver, proxy
}

func TestProxySelectionsSurviveCoreAndClientRestartOnAllPlatforms(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			root, core := t.TempDir(), newSelectionCore(goos)
			m, driver, _ := selectionManager(t, root, goos, core)
			request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
			if _, err := m.Start(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			session := *m.Status().ControllerSession
			client := newControllerClient(m.httpClient, session)
			for group, chosen := range map[string]string{manualGroup: "Node B", "GLOBAL": "Node B"} {
				if err := client.SelectProxy(context.Background(), group, chosen); err != nil {
					t.Fatal(err)
				}
				if err := m.RememberProxySelection(context.Background(), session.ID, request.Source.SubscriptionID, group, chosen); err != nil {
					t.Fatal(err)
				}
			}
			// Immediate durable writes must cover an unexpected core exit.
			driver.starts[0].crash(fmt.Errorf("fake core exit"))
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				state := m.Status()
				if state.State == StateRunning && state.PID == 1002 {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if state := m.Status(); state.State != StateRunning || state.PID != 1002 {
				t.Fatalf("recovery = %+v", state)
			}
			if core.now(manualGroup) != "Node B" || core.now("GLOBAL") != "Node B" {
				t.Fatal("crash recovery lost a manual choice")
			}
			if _, err := m.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Start(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if core.now(manualGroup) != "Node B" {
				t.Fatal("full restart lost selection")
			}
			if err := m.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			// A new Go manager cannot rely on any previous GUI memory.
			fresh, _, _ := selectionManager(t, root, goos, core)
			if _, err := fresh.Start(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if core.now(manualGroup) != "Node B" || core.now("GLOBAL") != "Node B" {
				t.Fatal("client restart lost selection")
			}
			for _, group := range core.puts {
				if group != manualGroup && group != "GLOBAL" {
					t.Fatalf("automatic group was pinned: %s", group)
				}
			}
		})
	}
}

func TestProxySelectionsAreIsolatedOnHotReloadAndRejectStaleWrites(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			core := newSelectionCore(goos)
			m, _, _ := selectionManager(t, t.TempDir(), goos, core)
			a := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
			if _, err := m.Start(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if err := m.SelectProxy(context.Background(), manualGroup, "Node B"); err != nil {
				t.Fatal(err)
			}
			oldSession := m.Status().ControllerSession.ID
			b := a
			b.Source.SubscriptionID = strings.Repeat("c", 32)
			if _, err := m.Start(context.Background(), b); err != nil {
				t.Fatal(err)
			}
			if m.Status().ControllerSession.ID != oldSession {
				t.Fatal("fixture must reuse hot-reload session")
			}
			if core.now(manualGroup) != "Node A" {
				t.Fatal("new subscription inherited another subscription's cached choice")
			}
			if err := m.RememberProxySelection(context.Background(), oldSession, a.Source.SubscriptionID, manualGroup, "Node B"); err == nil {
				t.Fatal("old subscription wrote into a reused session")
			}
			if err := m.SelectProxy(context.Background(), manualGroup, "Auto"); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Start(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if core.now(manualGroup) != "Node B" {
				t.Fatal("original subscription choice was lost")
			}
			a.Source.SubscriptionRevision = strings.Repeat("d", 64)
			if _, err := m.Start(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if core.now(manualGroup) != "Node B" {
				t.Fatal("refresh invalidated a still-valid choice")
			}
			if err := m.RememberProxySelection(context.Background(), "expired", a.Source.SubscriptionID, manualGroup, "Node B"); err == nil {
				t.Fatal("expired session accepted")
			}
			if err := m.RememberProxySelection(context.Background(), oldSession, a.Source.SubscriptionID, manualGroup, "Node A"); err == nil {
				t.Fatal("unconfirmed choice accepted")
			}
		})
	}
}

func TestProxySelectionsRestoreAfterTUNFinalReloadWithoutCoreCaching(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			core := newSelectionCore(goos)
			core.storeSelected = false
			m, _, _ := selectionManager(t, t.TempDir(), goos, core)
			request := runtimeRequest(t, runtimeconfig.ProxyModeTUN)
			if err := m.selections.merge(request.Source.SubscriptionID, map[string]string{manualGroup: "Node B"}); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Start(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if core.now(manualGroup) != "Node B" {
				t.Fatal("final TUN configuration reset the restored choice")
			}
		})
	}
}

func TestProxySelectionsSkipMissingNodesAndAutomaticGroups(t *testing.T) {
	core := newSelectionCore("darwin")
	core.groups[manualGroup] = proxySelectionState{Type: "Selector", All: []string{"Node A", "Node C"}}
	m, _, _ := selectionManager(t, t.TempDir(), "darwin", core)
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	request.Configuration = append(request.Configuration, []byte("proxy-groups:\n  - name: '"+manualGroup+"'\n    type: select\n    default-selected: Node C\n    proxies: [Node A, Node C]\n")...)
	if err := m.selections.merge(request.Source.SubscriptionID, map[string]string{manualGroup: "Deleted Node", "Removed Group": "Node B", "Auto": "Node A", "Fallback": "Node A", "Balance": "Node A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if core.now(manualGroup) != "Node C" {
		t.Fatal("missing choice did not fall back to the configured default")
	}
	for _, name := range []string{"Auto", "Fallback", "Balance"} {
		if core.now(name) != "Node B" {
			t.Fatal("restoration pinned an automatic group")
		}
	}
}

func TestProxySelectionFailureRollsBackReloadAndCannotEnableNewNetworking(t *testing.T) {
	core := newSelectionCore("windows")
	core.storeSelected = false
	m, _, proxy := selectionManager(t, t.TempDir(), "windows", core)
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := m.SelectProxy(context.Background(), manualGroup, "Node B"); err != nil {
		t.Fatal(err)
	}
	core.failReload = true
	if _, err := m.Start(context.Background(), request); err == nil {
		t.Fatal("reload failure was hidden")
	}
	if m.Status().State != StateRunning || core.now(manualGroup) != "Node B" {
		t.Fatal("rollback did not restore the previous choice")
	}
	if _, err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := proxy.applied
	core.failSelection = true
	if _, err := m.Start(context.Background(), request); err == nil {
		t.Fatal("restore failure was hidden")
	}
	if proxy.applied != before || m.Status().ControllerReady {
		t.Fatal("failed restoration enabled system proxy")
	}
}

func TestProxyChoicesWrittenByDirectAPIAreCapturedBeforeStopAndDelete(t *testing.T) {
	core := newSelectionCore("darwin")
	m, _, _ := selectionManager(t, t.TempDir(), "darwin", core)
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	client := newControllerClient(m.httpClient, *m.Status().ControllerSession)
	if err := client.SelectProxy(context.Background(), manualGroup, "Node B"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if core.now(manualGroup) != "Node B" {
		t.Fatal("stop failed to capture direct API selection")
	}
	if err := m.ForgetProxySelections(request.Source.SubscriptionID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	path, _ := m.selections.path(request.Source.SubscriptionID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stop recreated a deleted subscription's preferences")
	}
}

func TestProxySelectionSnapshotFailureKeepsSavedChoicesAndStillStops(t *testing.T) {
	core := newSelectionCore("windows")
	m, driver, proxy := selectionManager(t, t.TempDir(), "windows", core)
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := m.SelectProxy(context.Background(), manualGroup, "Node B"); err != nil {
		t.Fatal(err)
	}
	before := m.Status()
	path, _ := m.selections.path(request.Source.SubscriptionID)
	saved, _ := os.ReadFile(path)
	core.failSnapshot = true
	if _, err := m.Start(context.Background(), request); err == nil {
		t.Fatal("snapshot failure was hidden")
	}
	if after := m.Status(); after.State != StateRunning || after.GenerationID != before.GenerationID || core.now(manualGroup) != "Node B" {
		t.Fatal("snapshot failure replaced the healthy generation")
	}
	restores := proxy.restored
	if _, err := m.Stop(context.Background()); err == nil {
		t.Fatal("stop did not report the failed snapshot")
	}
	if !processExited(driver.starts[0]) || m.Status().ControllerReady || proxy.restored <= restores {
		t.Fatal("snapshot failure blocked network or core cleanup")
	}
	current, _ := os.ReadFile(path)
	if string(current) != string(saved) {
		t.Fatal("failed snapshot replaced valid choices")
	}
}

func TestProxySelectionFailedRollbackCannotOverwriteLastSavedChoice(t *testing.T) {
	core := newSelectionCore("darwin")
	core.storeSelected = false
	m, driver, _ := selectionManager(t, t.TempDir(), "darwin", core)
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	if _, err := m.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := m.SelectProxy(context.Background(), manualGroup, "Node B"); err != nil {
		t.Fatal(err)
	}
	core.failSelection = true
	if _, err := m.Start(context.Background(), request); err == nil {
		t.Fatal("failed restoration and rollback were hidden")
	}
	if !processExited(driver.starts[0]) || m.Status().ControllerReady {
		t.Fatal("failed rollback left an unconfirmed runtime active")
	}
	saved, err := m.selections.load(request.Source.SubscriptionID)
	if err != nil || saved[manualGroup] != "Node B" {
		t.Fatal("shutdown saved a fallback from an unconfirmed configuration")
	}
}

func TestSelectionStoreKeepsExactNamesAndRejectsInvalidOrCrossSubscriptionData(t *testing.T) {
	store := selectionStore{root: t.TempDir()}
	id := strings.Repeat("a", 32)
	values := map[string]string{" 🌐 Exact Group ": " 🇯🇵 Exact Node "}
	if err := store.merge(id, values); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.load(id)
	if err != nil || loaded[" 🌐 Exact Group "] != values[" 🌐 Exact Group "] {
		t.Fatalf("names changed: %#v %v", loaded, err)
	}
	if _, err := store.load("../outside"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	path, _ := store.path(id)
	previous, _ := os.ReadFile(path)
	if err := store.merge(id, map[string]string{"": "Node"}); err == nil {
		t.Fatal("invalid entry accepted")
	}
	current, _ := os.ReadFile(path)
	if string(current) != string(previous) {
		t.Fatal("invalid write replaced saved choices")
	}
	otherID := strings.Repeat("b", 32)
	otherPath, _ := store.path(otherID)
	if err := os.WriteFile(otherPath, previous, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.load(otherID); err == nil {
		t.Fatal("wrong subscription document accepted")
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.load(id); err == nil {
		t.Fatal("invalid schema accepted")
	}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := (selectionStore{root: blocked}).merge(id, values); err == nil {
		t.Fatal("failed persistence was hidden")
	}
}
