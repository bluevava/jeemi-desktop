package runtime

import (
	"net"
	"net/http"
	"net/url"
	"strconv"

	"jeemi/internal/runtimeconfig"
)

// NewDownloadClient follows the active listener, not unsaved or pending settings.
// It neither starts the core nor changes a selector or any system proxy setting.
func (m *Manager) NewDownloadClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	m.mu.RLock()
	if m.active != nil && m.status.ControllerReady && (m.status.State == StateRunning || m.status.State == StateReloading) {
		preferences := m.active.Preferences
		scheme := "http"
		if preferences.ListenerType == runtimeconfig.ListenerTypeSOCKS {
			scheme = "socks5"
		}
		transport.Proxy = http.ProxyURL(&url.URL{Scheme: scheme, Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(preferences.ListenPort))})
	}
	m.mu.RUnlock()
	return &http.Client{Transport: transport}
}
