package subscriptionformat

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
)

func renderMihomo(p *parsedDocument) ([]byte, string, error) {
	proxies := []map[string]any{}
	names := []string{}
	used := map[string]bool{"Proxy": true, "DIRECT": true, "REJECT": true, "REJECT-DROP": true, "PASS": true, "COMPATIBLE": true, "GLOBAL": true, "DNS": true}
	identities := map[string]bool{}
	required := ""
	for _, node := range p.Nodes {
		proxy := renderNode(node)
		// Include the original name in identity; credentials and all semantics
		// participate. Two accounts at the same server are distinct nodes.
		identity, _ := json.Marshal(proxy)
		if identities[string(identity)] {
			continue
		}
		identities[string(identity)] = true
		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = fmt.Sprintf("Node %d", len(names)+1)
		}
		original := name
		for suffix := 2; used[name]; suffix++ {
			name = fmt.Sprintf("%s (%d)", original, suffix)
		}
		if name != original {
			p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "node_renamed", Line: node.Line})
		}
		used[name] = true
		proxy["name"] = name
		proxies = append(proxies, proxy)
		names = append(names, name)
		if node.Transport.Network == "xhttp" || node.Encryption != "" {
			required = "v1.19.30"
		}
	}
	root := map[string]any{
		"proxies":      proxies,
		"proxy-groups": []any{map[string]any{"name": "Proxy", "type": "select", "proxies": names}},
		"rules":        []string{"MATCH,Proxy"},
	}
	dns := map[string]any{}
	if len(p.Nameservers) > 0 {
		dns["nameserver"] = p.Nameservers
	}
	if len(p.Bootstrap) > 0 {
		dns["default-nameserver"] = p.Bootstrap
	}
	policies, err := mergeDNSPolicies(p)
	if err != nil {
		return nil, "", err
	}
	general, nodePolicies := map[string][]string{}, map[string][]string{}
	for _, policy := range policies {
		general[policy.Pattern] = policy.Servers
		if policy.Nodes {
			nodePolicies[policy.Pattern] = policy.Servers
		}
	}
	if len(general) > 0 {
		dns["nameserver-policy"] = general
	}
	if len(nodePolicies) > 0 {
		dns["proxy-server-nameserver-policy"] = nodePolicies
		defaults := p.Nameservers
		if len(defaults) == 0 {
			defaults = []string{"system"}
		}
		dns["proxy-server-nameserver"] = defaults
	}
	if len(p.Hosts) > 0 {
		root["hosts"] = p.Hosts
		dns["use-hosts"] = true
	}
	if len(dns) > 0 {
		root["dns"] = dns
	}
	var ast yaml.Node
	if err := ast.Encode(root); err != nil {
		return nil, "", failure("invalid_output", 0, "")
	}
	output, err := document.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{&ast}})
	if err != nil || len(output) > MaxBytes {
		return nil, "", failure("output_limit", 0, "")
	}
	// Count the actual deduplicated output, not metadata or omitted nodes.
	p.ProxyCount = len(proxies)
	return output, required, nil
}

func renderNode(n proxyNode) map[string]any {
	p := map[string]any{"name": n.Name, "type": n.Protocol, "server": n.Server, "port": n.Port}
	if n.UDP != nil && n.Protocol != "tuic" {
		p["udp"] = *n.UDP
	}
	switch n.Protocol {
	case "ss":
		p["cipher"] = n.Cipher
		p["password"] = n.Password
	case "socks5":
		p["username"] = n.Username
		p["password"] = n.Password
	case "anytls":
		p["password"] = n.Password
		if n.IdleCheck != nil {
			p["idle-session-check-interval"] = *n.IdleCheck
		}
		if n.IdleTimeout != nil {
			p["idle-session-timeout"] = *n.IdleTimeout
		}
		if n.MinIdle != nil {
			p["min-idle-session"] = *n.MinIdle
		}
	case "tuic":
		p["uuid"] = n.UUID
		p["password"] = n.Password
		if n.Congestion != "" {
			p["congestion-controller"] = n.Congestion
		}
		if n.UDPRelayMode != "" {
			p["udp-relay-mode"] = n.UDPRelayMode
		}
		if n.ReduceRTT != nil {
			p["reduce-rtt"] = *n.ReduceRTT
		}
		if n.Heartbeat != nil {
			p["heartbeat-interval"] = *n.Heartbeat
		}
	case "vless":
		p["uuid"] = n.UUID
		p["network"] = n.Transport.Network
		if n.Flow != "" {
			p["flow"] = n.Flow
		}
		if n.PacketEncoding != "" && n.PacketEncoding != "none" {
			p["packet-encoding"] = n.PacketEncoding
		}
		if n.Encryption != "" {
			p["encryption"] = n.Encryption
		}
		switch n.Transport.Network {
		case "ws":
			opts := map[string]any{}
			if n.Transport.Path != "" {
				opts["path"] = n.Transport.Path
			}
			if n.Transport.Host != "" {
				opts["headers"] = map[string]string{"Host": n.Transport.Host}
			}
			p["ws-opts"] = opts
		case "grpc":
			p["grpc-opts"] = map[string]string{"grpc-service-name": n.Transport.ServiceName}
		case "xhttp":
			opts := map[string]any{"no-grpc-header": false}
			if n.Transport.Path != "" {
				opts["path"] = n.Transport.Path
			}
			if n.Transport.Host != "" {
				opts["host"] = n.Transport.Host
			}
			if n.Transport.Mode != "" {
				opts["mode"] = n.Transport.Mode
			}
			p["xhttp-opts"] = opts
		}
	case "snell":
		p["psk"] = n.PSK
		p["version"] = n.SnellVersion
		if n.ObfsMode != "" && n.ObfsMode != "none" {
			opts := map[string]string{"mode": n.ObfsMode}
			if n.ObfsHost != "" {
				opts["host"] = n.ObfsHost
			}
			p["obfs-opts"] = opts
		}
	}
	if n.TLS.Enabled {
		if n.Protocol == "vless" {
			p["tls"] = true
		}
		if n.TLS.ServerName != "" {
			key := "sni"
			if n.Protocol == "vless" {
				key = "servername"
			}
			p[key] = n.TLS.ServerName
		}
		if n.TLS.LeafSHA256 != "" {
			p["fingerprint"] = n.TLS.LeafSHA256
		}
		if n.TLS.ClientFingerprint != "" {
			p["client-fingerprint"] = n.TLS.ClientFingerprint
		}
		if len(n.TLS.ALPN) > 0 {
			p["alpn"] = n.TLS.ALPN
		}
		if n.TLS.SkipVerify != nil {
			p["skip-cert-verify"] = *n.TLS.SkipVerify
		}
		if n.TLS.RealityPublicKey != "" {
			p["reality-opts"] = map[string]string{"public-key": n.TLS.RealityPublicKey, "short-id": n.TLS.RealityShortID}
		}
	}
	return p
}
