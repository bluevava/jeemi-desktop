package runtime

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/dnsquery"

	"golang.org/x/net/proxy"
)

func (m *Manager) CancelDNSQuery(id string) {
	if len(id) == 0 || len(id) > 80 {
		return
	}
	m.dnsMu.Lock()
	defer m.dnsMu.Unlock()
	if m.dnsID == id && m.dnsCancel != nil {
		m.dnsCancel()
		return
	}
	// Wails calls run concurrently: cancellation can arrive before QueryDNS
	// registers its context. Keep a small bounded set of cancelled request IDs.
	if m.dnsCancelled == nil {
		m.dnsCancelled = map[string]bool{}
	}
	if len(m.dnsCancelled) >= 32 {
		for previous := range m.dnsCancelled {
			delete(m.dnsCancelled, previous)
			break
		}
	}
	m.dnsCancelled[id] = true
}

func (m *Manager) cancelActiveDNSQuery() {
	m.dnsMu.Lock()
	defer m.dnsMu.Unlock()
	if m.dnsCancel != nil {
		m.dnsCancel()
	}
}

// QueryDNS serializes tool requests, without holding the lifecycle lock during
// network IO. Only the private tool selector is changed, never user selectors.
func (m *Manager) QueryDNS(ctx context.Context, input dnsquery.Request) (dnsquery.Response, error) {
	select {
	case <-m.shutdown:
		return dnsquery.Response{}, errors.New("cancelled")
	default:
	}
	domain, err := dnsquery.Domain(input.Domain)
	if err != nil {
		return dnsquery.Response{}, err
	}
	if len(input.ID) < 1 || len(input.ID) > 80 {
		return dnsquery.Response{}, errors.New("invalid_request")
	}
	m.dnsMu.Lock()
	if m.dnsCancelled[input.ID] {
		delete(m.dnsCancelled, input.ID)
		m.dnsMu.Unlock()
		return dnsquery.Response{}, errors.New("cancelled")
	}
	if m.dnsCancel != nil {
		m.dnsMu.Unlock()
		return dnsquery.Response{}, errors.New("busy")
	}
	ctx, cancel := context.WithTimeout(ctx, dnsquery.Timeout)
	m.dnsID, m.dnsCancel = input.ID, cancel
	m.dnsMu.Unlock()
	defer func() {
		cancel()
		m.dnsMu.Lock()
		m.dnsID, m.dnsCancel = "", nil
		m.dnsMu.Unlock()
	}()

	m.mu.RLock()
	var active Generation
	running := m.status.State == StateRunning && m.status.ControllerReady && m.active != nil && m.client != nil
	if running {
		active = *m.active
	}
	client := m.client
	localReady := m.process == nil && m.recoveryBlocked == nil && !m.status.TUNEnabled && !m.status.DesiredRunning
	m.mu.RUnlock()
	response := dnsquery.Response{Domain: domain, QueriedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	var group sync.WaitGroup
	queryLocal := func(server string) dnsquery.Result {
		var dial dnsquery.DialContext
		if running {
			var err error
			dial, err = dnsToolDial(active.DNS.DirectAddress, active.DNS)
			if err != nil {
				return dnsquery.Failed(server, "direct", "direct_unavailable")
			}
		} else if !localReady {
			return dnsquery.Failed(server, "direct", "direct_unavailable")
		}
		return dnsquery.Query(ctx, domain, server, "direct", dial)
	}
	group.Add(1)
	go func() { defer group.Done(); response.Direct = queryLocal(input.DirectDNS) }()
	if !input.CustomProxy {
		group.Add(1)
		go func() { defer group.Done(); response.Custom = queryLocal(input.CustomDNS) }()
	}
	proxyCode, node := "proxy_invalid", ""
	var proxyDial dnsquery.DialContext
	if running && active.DNS.MatchTarget != "" {
		preflightCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		states, fetchErr := client.proxySelections(preflightCtx)
		if fetchErr == nil && len(active.DNS.ProviderNames) > 0 {
			// Include declared provider leaves and reject ambiguous duplicate
			// names. The hidden group must never resolve the same name to a
			// different provider than the original MATCH selector.
			fetchErr = client.addDNSProviderLeaves(preflightCtx, states, active.DNS.ProviderNames)
		}
		if fetchErr == nil {
			node, proxyCode = dnsProxyLeaf(active.DNS.MatchTarget, states)
			if proxyCode == "" {
				tool, found := states[dnstool.Group]
				if !found || !slices.Contains(tool.All, node) || client.SelectProxy(preflightCtx, dnstool.Group, node) != nil {
					proxyCode = "proxy_invalid"
				} else {
					proxyDial, err = dnsToolDial(active.DNS.ProxyAddress, active.DNS)
					if err != nil {
						proxyCode = "proxy_invalid"
					}
				}
			}
		}
		stop()
	}
	queryProxy := func(server string) dnsquery.Result {
		if _, _, err := dnsquery.Server(server); err != nil {
			return dnsquery.Failed(server, "proxy", "invalid_server")
		}
		if proxyCode != "" {
			return dnsquery.Failed(server, "proxy", proxyCode)
		}
		result := dnsquery.Query(ctx, domain, server, "proxy", proxyDial)
		result.Selector, result.Node = active.DNS.MatchTarget, node
		for _, record := range result.Records {
			if record.Error == "query_failed" || record.Error == "timeout" {
				// No direct fallback after a proxy connection failure. DNS RCODEs
				// such as NXDOMAIN remain DNS results, not proxy-health claims.
				result.Error = "proxy_invalid"
			}
		}
		return result
	}
	group.Add(1)
	go func() { defer group.Done(); response.Proxy = queryProxy(input.ProxyDNS) }()
	if input.CustomProxy {
		group.Add(1)
		go func() { defer group.Done(); response.Custom = queryProxy(input.CustomDNS) }()
	}
	group.Wait()
	m.mu.RLock()
	unchanged := running && m.status.State == StateRunning && m.status.ControllerReady && m.active != nil && m.active.ID == active.ID
	stillLocal := !running && m.process == nil && m.recoveryBlocked == nil && !m.status.TUNEnabled && !m.status.DesiredRunning
	m.mu.RUnlock()
	if !unchanged && !stillLocal {
		response.Proxy = dnsquery.Failed(input.ProxyDNS, "proxy", "session_changed")
		response.Direct = dnsquery.Failed(input.DirectDNS, "direct", "session_changed")
		route := "direct"
		if input.CustomProxy {
			route = "proxy"
		}
		response.Custom = dnsquery.Failed(input.CustomDNS, route, "session_changed")
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return dnsquery.Response{}, errors.New("cancelled")
	}
	return response, nil
}

func dnsToolDial(address string, session dnstool.Session) (dnsquery.DialContext, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" || len(session.Password) < 32 {
		return nil, errors.New("proxy_invalid")
	}
	dialer, err := proxy.SOCKS5("tcp", address, &proxy.Auth{User: session.Username, Password: session.Password}, &net.Dialer{Timeout: dnsquery.Timeout})
	if err != nil {
		return nil, err
	}
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("proxy_invalid")
	}
	return contextDialer.DialContext, nil
}

func dnsProxyLeaf(target string, states map[string]proxySelectionState) (string, string) {
	visited := map[string]bool{}
	for depth := 0; depth < 32; depth++ {
		if visited[target] {
			return "", "proxy_invalid"
		}
		visited[target] = true
		state, found := states[target]
		if !found {
			return "", "missing_proxy"
		}
		switch strings.ToLower(state.Type) {
		case "selector", "urltest", "fallback":
			if state.Now == "" || !slices.Contains(state.All, state.Now) {
				return "", "proxy_invalid"
			}
			target = state.Now
		case "loadbalance", "relay", "dnsambiguous":
			return "", "ambiguous_proxy"
		case "", "direct", "reject", "rejectdrop", "compatible", "pass", "dns":
			return "", "proxy_invalid"
		default:
			if state.Now != "" || len(state.All) > 0 {
				return "", "ambiguous_proxy"
			}
			return target, ""
		}
	}
	return "", "proxy_invalid"
}

func (c *controllerClient) addDNSProviderLeaves(ctx context.Context, states map[string]proxySelectionState, providerNames []string) error {
	var response struct {
		Providers map[string]struct {
			Proxies []struct {
				Name string `json:"name"`
				proxySelectionState
			} `json:"proxies"`
		} `json:"providers"`
	}
	if err := c.doJSONWithLimit(ctx, http.MethodGet, "/providers/proxies", nil, &response, 16<<20); err != nil {
		return err
	}
	for _, name := range providerNames {
		provider, found := response.Providers[name]
		if !found {
			return errors.New("missing_proxy")
		}
		for _, leaf := range provider.Proxies {
			if _, found := states[leaf.Name]; found {
				states[leaf.Name] = proxySelectionState{Type: "DNSAmbiguous"}
				continue
			}
			states[leaf.Name] = leaf.proxySelectionState
		}
	}
	return nil
}
