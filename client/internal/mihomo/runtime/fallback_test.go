package runtime

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/runtimeconfig"
)

func TestFallbackChangeHotReloadsAndFailedValidationKeepsHealthySource(t *testing.T) {
	driver := &fakeRuntimeDriver{}
	manager, err := NewManager(Options{DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: &fakeSystemProxy{}, HTTPClient: healthyHTTPClient(), GOOS: "windows"})
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeRequest(t, runtimeconfig.ProxyModeSystemProxy)
	request.Configuration = []byte("proxy-groups: [{name: Main, type: select, proxies: [DIRECT]}]\nrules: [\"MATCH,Main\"]\n")
	started, err := manager.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = manager.Stop(context.Background()) })
	candidate := request
	candidate.Configuration, _, err = fallbackoverride.Apply(request.Configuration, fallbackoverride.Selection{Mode: fallbackoverride.ModeDirect})
	if err != nil {
		t.Fatal(err)
	}
	candidate.Source.FallbackOverrideRevision = 1
	reloaded, err := manager.Start(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.PID != started.PID || reloaded.GenerationID == started.GenerationID || reloaded.Source.FallbackOverrideRevision != 1 || len(driver.starts) != 1 {
		t.Fatalf("fallback change did not use a new generation on the same process: %+v", reloaded)
	}
	view, err := manager.ActiveConfiguration()
	if err != nil || !strings.Contains(string(view.Contents), "MATCH,DIRECT") {
		t.Fatalf("fallback did not reach active generation: %s %v", view.Contents, err)
	}
	driver.mu.Lock()
	driver.validateErr = context.Canceled
	driver.mu.Unlock()
	candidate.Configuration = request.Configuration
	candidate.Source.FallbackOverrideRevision = 2
	failed, err := manager.Start(context.Background(), candidate)
	if err == nil || failed.GenerationID != reloaded.GenerationID || failed.Source.FallbackOverrideRevision != 1 || failed.State != StateRunning {
		t.Fatalf("failed change replaced healthy generation: %+v %v", failed, err)
	}
}
