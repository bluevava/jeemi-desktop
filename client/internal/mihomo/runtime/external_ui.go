package runtime

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"jeemi/internal/runtimeconfig"
)

// mihomo registers /ui only while creating its controller router at startup.
// PUT /configs cannot add, remove or repoint that static-file route.
func (m *Manager) ExternalUIRequiresRestart(next runtimeconfig.Preferences) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.active == nil || m.status.State != StateRunning {
		return false
	}
	previous := m.active.Preferences
	return previous.ExternalUIEnabled != next.ExternalUIEnabled ||
		(next.ExternalUIEnabled && previous.ExternalUIVersion != next.ExternalUIVersion)
}

// OpenExternalUI keeps the port/secret within Go and opens the current healthy
// session atomically with respect to stops/reloads. The URL is never persisted
// or returned over a Wails binding; credentials are only in the local fragment.
func (m *Manager) OpenExternalUI(open func(string)) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.mu.RLock()
	if m.active == nil || !m.active.Preferences.ExternalUIEnabled || m.status.State != StateRunning || !m.status.ControllerReady {
		m.mu.RUnlock()
		return fmt.Errorf("external UI requires an enabled, running mihomo session")
	}
	session := m.active.Session
	m.mu.RUnlock()
	address, err := externalUIURL(session)
	if err != nil {
		return err
	}
	open(address)
	return nil
}

func externalUIURL(session ControllerSession) (string, error) {
	address, err := url.Parse(session.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid external UI controller session")
	}
	ip := net.ParseIP(address.Hostname())
	port, portErr := strconv.Atoi(address.Port())
	if address.Scheme != "http" || ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 || session.Secret == "" || address.User != nil || address.RawQuery != "" || address.Fragment != "" || (address.Path != "" && address.Path != "/") {
		return "", fmt.Errorf("invalid external UI controller session")
	}
	address.Path = "/ui/"
	address.Fragment = "/setup?" + url.Values{
		"protocol": {"http"}, "hostname": {address.Hostname()}, "port": {address.Port()}, "secret": {session.Secret},
	}.Encode()
	return address.String(), nil
}
