package runtime

import (
	"context"
	"encoding/json"
	"io"
	"jeemi/internal/runtimeconfig"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type fakeManagedDriver struct {
	fakeRuntimeDriver
	events *[]string
}

func (d *fakeManagedDriver) ActivateTUN(context.Context, runtimeconfig.Preferences) error {
	*d.events = append(*d.events, "tun")
	return nil
}
func (d *fakeManagedDriver) DeactivateNetwork(context.Context) error {
	*d.events = append(*d.events, "restore")
	return nil
}
func (d *fakeManagedDriver) TUNDevice() string { return "utun7" }
func (d *fakeManagedDriver) StageConfiguration(context.Context, string) (string, error) {
	*d.events = append(*d.events, "stage")
	return "/protected/config.yaml", nil
}
func TestManagedTUNStagesAfterHealthAndRestoresBeforeDisabling(t *testing.T) {
	events := []string{}
	driver := &fakeManagedDriver{events: &events}
	base := healthyHTTPClient().Transport
	client := &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.Path == "/version" {
			events = append(events, "health")
		}
		if request.Method == http.MethodPut && request.URL.Path == "/configs" {
			data, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(data), "/protected/config.yaml") {
				t.Error("unprotected configuration path sent to core")
			}
			events = append(events, "reload")
		}
		if request.Method == http.MethodPatch && request.URL.Path == "/configs" {
			var body struct {
				TUN *struct {
					Enable bool `json:"enable"`
				} `json:"tun"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body.TUN != nil && !body.TUN.Enable {
				events = append(events, "disable")
			}
		}
		return base.RoundTrip(request)
	})}
	manager, err := NewManager(Options{DataDirectory: filepath.Join(t.TempDir(), "jeemi_data"), Driver: driver, SystemProxy: &fakeSystemProxy{}, HTTPClient: client, GOOS: "darwin", ObserveTUN: func(_ context.Context, name string) error {
		if name != "utun7" {
			t.Error(name)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.Start(context.Background(), runtimeRequest(t, runtimeconfig.ProxyModeTUN))
	if err != nil {
		t.Fatal(err)
	}
	if status.TUNDevice != "utun7" || !status.TUNEnabled {
		t.Fatal(status.TUNDevice)
	}
	if _, err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	index := func(want string) int {
		for i, event := range events {
			if event == want {
				return i
			}
		}
		return -1
	}
	if index("health") < 0 || index("health") > index("stage") || index("stage") > index("tun") || index("restore") < 0 || index("restore") > index("disable") {
		t.Fatal(events)
	}
}
