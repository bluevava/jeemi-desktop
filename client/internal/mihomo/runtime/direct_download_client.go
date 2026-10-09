package runtime

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// NewDirectDownloadClient uses the active generation's authenticated DIRECT
// inbound. This bypasses proxy nodes even with TUN enabled, without changing
// the core's routing mode or selectors. Nil means no healthy route is available.
func (m *Manager) NewDirectDownloadClient() *http.Client {
	m.mu.RLock()
	if !m.directDownloadReadyLocked() {
		m.mu.RUnlock()
		return nil
	}
	active := *m.active
	m.mu.RUnlock()
	dial, err := dnsToolDial(active.DNS.DirectAddress, active.DNS)
	if err != nil {
		return nil
	}
	current := func() bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.directDownloadReadyLocked() && m.active.ID == active.ID
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if !current() {
			return nil, errors.New("mihomo direct download session is unavailable")
		}
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, errors.New("mihomo direct download connection failed")
		}
		if !current() {
			_ = conn.Close()
			return nil, errors.New("mihomo direct download session changed")
		}
		return conn, nil
	}
	return &http.Client{Transport: transport}
}

func (m *Manager) directDownloadReadyLocked() bool {
	return m.active != nil && m.client != nil && m.status.ControllerReady &&
		(m.status.State == StateRunning || m.status.State == StateReloading)
}
