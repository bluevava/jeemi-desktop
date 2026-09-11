package runtime

import (
	"net/http"
	"testing"

	"jeemi/internal/runtimeconfig"
)

func TestDownloadClientUsesHealthyActiveListener(t *testing.T) {
	for _, listener := range []string{runtimeconfig.ListenerTypeHTTP, runtimeconfig.ListenerTypeMixed, runtimeconfig.ListenerTypeSOCKS} {
		manager, err := NewManager(Options{DataDirectory: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		manager.active = &Generation{Preferences: runtimeconfig.Preferences{ListenerType: listener, ListenPort: 9876}}
		manager.status = Status{State: StateRunning, ControllerReady: true}
		request, _ := http.NewRequest(http.MethodGet, "https://assets.example/icon", nil)
		transport := manager.NewDownloadClient().Transport.(*http.Transport)
		proxy, err := transport.Proxy(request)
		expected := "http"
		if listener == runtimeconfig.ListenerTypeSOCKS {
			expected = "socks5"
		}
		if err != nil || proxy == nil || proxy.Scheme != expected || proxy.Host != "127.0.0.1:9876" || proxy.User != nil {
			t.Fatalf("incorrect active proxy: %v", err)
		}
		if !transport.DisableKeepAlives {
			t.Fatal("one-shot client retained idle connections")
		}
		manager.status = Status{State: StateStarting}
		inactive, _ := manager.NewDownloadClient().Transport.(*http.Transport).Proxy(request)
		if inactive != nil && inactive.Host == proxy.Host {
			t.Fatal("unhealthy listener was reused")
		}
	}
}
