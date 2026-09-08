package runtime

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"jeemi/internal/runtimeconfig"
)

type fallbackConnectionsAPI struct {
	mu          sync.Mutex
	events      []string
	failCapture bool
	failDelete  bool
	failReload  bool
	reloaded    bool
}

func (a *fallbackConnectionsAPI) client() *http.Client {
	healthy := healthyHTTPClient()
	return &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.events = append(a.events, request.Method+" "+request.URL.Path)
		if request.Header.Get("Authorization") == "" {
			return nil, context.Canceled
		}
		code, body := http.StatusNoContent, ""
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/connections":
			code = http.StatusOK
			// Include exact, nested, similarly named, direct, empty and duplicate IDs.
			body = `{"connections":[{"id":"old","chains":["Leaf","Main"]},{"id":"nested","chains":["Leaf","Auto","Main"]},{"id":"other","chains":["Main extra"]},{"id":"direct","chains":["DIRECT"]},{"id":"","chains":["Main"]},{"id":"old","chains":["Main"]}]}`
			if a.reloaded {
				body = strings.TrimSuffix(body, "]}") + `,{"id":"during-reload","chains":["Main"]}]}`
			}
			if a.failCapture {
				code = http.StatusServiceUnavailable
			}
		case request.Method == http.MethodDelete:
			if a.failDelete && request.URL.Path == "/connections/old" {
				code = http.StatusServiceUnavailable
			}
		case request.Method == http.MethodPut && request.URL.Path == "/configs":
			a.reloaded = true
			if a.failReload {
				a.failReload = false // Rollback remains healthy.
				code = http.StatusServiceUnavailable
			}
		default:
			return healthy.Do(request)
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

func (a *fallbackConnectionsAPI) resetEvents() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = nil
}

func (a *fallbackConnectionsAPI) closed() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result []string
	for _, event := range a.events {
		if strings.HasPrefix(event, "DELETE ") {
			result = append(result, event)
		}
	}
	slices.Sort(result)
	return result
}

func fallbackResetRuntime(t *testing.T, outboundMode, target string) (*Manager, *fakeRuntimeDriver, *fallbackConnectionsAPI, StartRequest) {
	t.Helper()
	driver, api := &fakeRuntimeDriver{}, &fallbackConnectionsAPI{}
	manager, err := NewManager(Options{DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: &fakeSystemProxy{}, HTTPClient: api.client(), GOOS: "windows"})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	request.Preferences.OutboundMode = outboundMode
	request.Configuration = []byte("proxy-groups: [{name: Main, type: select, proxies: [DIRECT]}, {name: Other, type: select, proxies: [DIRECT]}]\nrules: [\"MATCH," + target + "\"]\n")
	if _, err := manager.Start(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = manager.Stop(context.Background()) })
	api.resetEvents()
	return manager, driver, api, request
}

func TestFallbackResetUsesSettingAndPreviousActualTarget(t *testing.T) {
	for _, test := range []struct {
		name, mode, old, next, outbound string
		closed                          []string
	}{
		{"off", "off", "Main", "Other", "rule", nil},
		{"selector", "selector", "Main", "Other", "rule", []string{"DELETE /connections/during-reload", "DELETE /connections/nested", "DELETE /connections/old"}},
		{"all", "all", "Main", "Other", "rule", []string{"DELETE /connections/direct", "DELETE /connections/during-reload", "DELETE /connections/nested", "DELETE /connections/old", "DELETE /connections/other"}},
		{"direct to selector", "selector", "DIRECT", "Other", "rule", []string{"DELETE /connections/direct"}},
		{"selector to direct", "selector", "Main", "DIRECT", "rule", []string{"DELETE /connections/during-reload", "DELETE /connections/nested", "DELETE /connections/old"}},
		{"unchanged effective target", "all", "Main", "Main", "rule", nil},
		{"global", "all", "Main", "Other", "global", nil},
		{"direct mode", "all", "Main", "Other", "direct", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, _, api, original := fallbackResetRuntime(t, test.outbound, test.old)
			next := original
			next.Configuration = []byte(strings.ReplaceAll(string(original.Configuration), "MATCH,"+test.old, "MATCH,"+test.next))
			next.Source.FallbackOverrideRevision = 1
			plan := manager.PrepareFallbackConnectionReset(original.Source.SubscriptionID, next.Configuration, test.mode)
			if len(api.closed()) != 0 {
				t.Fatal("capture closed connections before reload")
			}
			if test.closed == nil && plan != nil {
				t.Fatal("unnecessary capture for inactive/unchanged fallback")
			}
			if _, err := manager.Start(context.Background(), next); err != nil {
				t.Fatal(err)
			}
			if err := manager.CompleteFallbackConnectionReset(context.Background(), plan, 1); err != nil {
				t.Fatal(err)
			}
			if got := api.closed(); !slices.Equal(got, test.closed) {
				t.Fatalf("closed %v, want %v", got, test.closed)
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			capture := slices.Index(api.events, "GET /connections")
			reload := slices.Index(api.events, "PUT /configs")
			health := slices.Index(api.events, "GET /version")
			if test.closed == nil && capture >= 0 {
				t.Fatal("disabled reset queried connections")
			}
			if test.closed != nil && !(reload >= 0 && reload < health && health < capture) {
				t.Fatalf("capture/reload/health ordering: %v", api.events)
			}
			for i, event := range api.events {
				if strings.HasPrefix(event, "DELETE ") && i <= health {
					t.Fatal("connections closed before successful reload health check")
				}
			}
			// Use one post-reload snapshot; new connections cannot enter the deletion batch.
			if capture >= 0 && slices.Contains(api.events[capture+1:], "GET /connections") {
				t.Fatal("reset kept recapturing connections during deletion")
			}
		})
	}
}

func TestFallbackResetSkipsFailedDeferredAndSupersededGenerations(t *testing.T) {
	for _, change := range []string{"deferred", "validation failure", "reload failure", "restart", "different subscription", "different revision", "different target", "stopped"} {
		t.Run(change, func(t *testing.T) {
			manager, driver, api, next := fallbackResetRuntime(t, "rule", "Main")
			next.Configuration = []byte(strings.ReplaceAll(string(next.Configuration), "MATCH,Main", "MATCH,Other"))
			next.Source.FallbackOverrideRevision = 1
			plan := manager.PrepareFallbackConnectionReset(next.Source.SubscriptionID, next.Configuration, "all")
			switch change {
			case "validation failure":
				driver.validateErr = context.Canceled
			case "reload failure":
				api.failReload = true
			case "restart", "stopped":
				_, _ = manager.Stop(context.Background())
			case "different subscription":
				next.Source.SubscriptionID = strings.Repeat("c", 32)
			case "different revision":
				next.Source.FallbackOverrideRevision = 2
			case "different target":
				next.Configuration = []byte(strings.ReplaceAll(string(next.Configuration), "MATCH,Other", "MATCH,DIRECT"))
			}
			if change != "deferred" && change != "stopped" {
				_, err := manager.Start(context.Background(), next)
				if (change == "validation failure" || change == "reload failure") != (err != nil) {
					t.Fatalf("unexpected activation result: %v", err)
				}
			}
			_ = manager.CompleteFallbackConnectionReset(context.Background(), plan, 1)
			if got := api.closed(); len(got) > 0 {
				t.Fatalf("closed connections without matching successful activation: %v", got)
			}
			if slices.Contains(api.events, "GET /connections") {
				t.Fatal("read connections without matching successful activation")
			}
		})
	}
}

func TestFallbackResetFailureDoesNotUndoAppliedRouting(t *testing.T) {
	for _, captureFailure := range []bool{true, false} {
		t.Run(map[bool]string{true: "capture failure", false: "partial deletion failure"}[captureFailure], func(t *testing.T) {
			manager, _, api, next := fallbackResetRuntime(t, "rule", "Main")
			api.failCapture, api.failDelete = captureFailure, !captureFailure
			next.Configuration = []byte(strings.ReplaceAll(string(next.Configuration), "MATCH,Main", "MATCH,Other"))
			next.Source.FallbackOverrideRevision = 1
			plan := manager.PrepareFallbackConnectionReset(next.Source.SubscriptionID, next.Configuration, "selector")
			if _, err := manager.Start(context.Background(), next); err != nil {
				t.Fatal(err)
			}
			if err := manager.CompleteFallbackConnectionReset(context.Background(), plan, 1); err == nil {
				t.Fatal("reset failure was not reported separately")
			}
			if status := manager.Status(); status.State != StateRunning || status.Source.FallbackOverrideRevision != 1 || status.LastError != nil {
				t.Fatalf("reset failure undid healthy routing: %+v", status)
			}
			if !captureFailure && len(api.closed()) != 3 {
				t.Fatal("one failed deletion prevented the rest of the batch")
			}
		})
	}
}
