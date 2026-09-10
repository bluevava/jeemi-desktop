package subscriptionformat

import (
	"encoding/hex"
	"strings"
)

func validateNativeProxy(node map[string]any, kind string, definition nativeProtocol, line int) (string, error) {
	invalid := func() (string, error) { return "", failure("invalid_field", line, "node") }
	for _, key := range strings.Fields(definition.required) {
		if nativeText(node, key) == "" {
			return invalid()
		}
	}
	for _, key := range []string{"name", "username", "password", "uuid", "cipher", "psk", "token"} {
		if value, ok := node[key].(string); ok && !validText(value, maxFieldBytes) {
			return invalid()
		}
	}
	if name := nativeText(node, "name"); len(name) > 1024 {
		return invalid()
	}
	if nativeText(node, "client-fingerprint") != "" && !oneOf(kind, "ss", "vmess", "vless", "trojan", "anytls") {
		return "unsupported_node_fields", nil
	}
	if _, exists := node["tlsmirror-opts"]; exists && kind != "vmess" {
		return "unsupported_tls_combination", nil
	}
	if _, supportsCertificate := definition.fields["certificate"]; supportsCertificate && kind != "masque" {
		if (nativeText(node, "certificate") != "") != (nativeText(node, "private-key") != "") {
			return invalid()
		}
	}
	peers, _ := node["peers"].([]any)
	needsEndpoint := !definition.noServer && !(kind == "wireguard" && len(peers) > 0)
	if needsEndpoint && nativeText(node, "server") == "" {
		return invalid()
	}
	if _, present := node["port"]; !present {
		switch kind {
		case "ssh":
			node["port"] = 22
		case "openvpn":
			node["port"] = 1194
		}
	}
	if port, present := node["port"].(int); present {
		if port < 1 || port > 65535 {
			return invalid()
		}
	} else if needsEndpoint && !(kind == "mieru" && nativeText(node, "port-range") != "") {
		return invalid()
	}
	if kind == "mieru" && nativeText(node, "port-range") != "" {
		if _, present := node["port"]; present {
			return invalid()
		}
	}
	if kind == "vmess" || kind == "vless" {
		if !validUUID(nativeText(node, "uuid")) {
			return invalid()
		}
		if kind == "vmess" {
			if _, present := node["alterId"]; !present {
				node["alterId"] = 0
			}
			if _, present := node["cipher"]; !present {
				node["cipher"] = "auto"
			}
		}
	}
	if kind == "tuic" {
		if nativeText(node, "token") != "" {
			if nativeText(node, "uuid") != "" || nativeText(node, "password") != "" {
				return invalid()
			}
		} else if !validUUID(nativeText(node, "uuid")) || nativeText(node, "password") == "" {
			return invalid()
		}
		if value, present := node["udp"].(bool); present && !value {
			return "unsupported_udp_setting", nil
		}
		delete(node, "udp")
		if !oneOf(nativeText(node, "udp-relay-mode"), "", "native", "quic") || !oneOf(nativeText(node, "congestion-controller"), "", "cubic", "new_reno", "bbr") {
			return invalid()
		}
	}
	if kind == "anytls" {
		for _, key := range []string{"idle-session-check-interval", "idle-session-timeout"} {
			if value, ok := node[key].(int); ok && (value <= 0 || value > 86400) {
				return invalid()
			}
		}
	}
	if kind == "snell" {
		if _, present := node["version"]; !present {
			node["version"] = 1
		}
		version := node["version"].(int)
		if version < 1 || version > 5 {
			return "unsupported_protocol_version", nil
		}
		if version < 3 && nativeFlag(node, "udp") {
			return invalid()
		}
	}
	if kind == "openvpn" && !(nativeText(node, "username") != "" && nativeText(node, "password") != "") && !(nativeText(node, "cert") != "" && nativeText(node, "key") != "") {
		return invalid()
	}
	if kind == "wireguard" {
		if len(peers) == 0 && nativeText(node, "public-key") == "" {
			return invalid()
		}
		for _, raw := range peers {
			peer, ok := raw.(map[string]any)
			if !ok || nativeText(peer, "public-key") == "" {
				return invalid()
			}
			if _, err := normalizeServer(nativeText(peer, "server")); err != nil {
				return invalid()
			}
			port, ok := peer["port"].(int)
			if !ok || port < 1 || port > 65535 {
				return invalid()
			}
		}
	}
	if definition.networks != "" {
		network := nativeText(node, "network")
		if network == "" && kind != "masque" {
			node["network"] = "tcp"
			network = "tcp"
		}
		if network != "" && !oneOf(network, strings.Fields(definition.networks)...) {
			return "unsupported_transport", nil
		}
		for _, transport := range []string{"ws", "http", "h2", "grpc", "xhttp", "mkcp", "mekya"} {
			if _, exists := node[transport+"-opts"]; exists && network != transport {
				return "unsupported_transport", nil
			}
		}
	}
	if kind == "vless" {
		if nativeText(node, "encryption") == "none" {
			delete(node, "encryption")
		}
		if !oneOf(nativeText(node, "flow"), "", "xtls-rprx-vision") || !oneOf(nativeText(node, "packet-encoding"), "", "none", "xudp", "packetaddr") {
			return "unsupported_transport", nil
		}
	}
	if definition.tlsAlways {
		if enabled, exists := node["tls"].(bool); exists && !enabled {
			return "unsupported_tls_combination", nil
		}
		delete(node, "tls")
	}
	if reality, ok := node["reality-opts"].(map[string]any); ok {
		publicKey, valid := decodeBase64(nativeText(reality, "public-key"))
		shortID, err := hex.DecodeString(nativeText(reality, "short-id"))
		if !oneOf(kind, "vmess", "vless", "trojan") || !valid || len(publicKey) != 32 || err != nil || len(shortID) > 8 || nativeText(node, nativeSNIKey(kind)) == "" {
			return invalid()
		}
		if nativeText(node, "fingerprint") != "" {
			return "unsupported_tls_combination", nil
		}
		if _, explicit := node["skip-cert-verify"]; explicit {
			return "unsupported_tls_combination", nil
		}
		if !definition.tlsAlways && !nativeFlag(node, "tls") {
			return "unsupported_tls_combination", nil
		}
	}
	if !definition.tlsAlways && !nativeFlag(node, "tls") && oneOf(kind, "vmess", "vless", "http", "socks5") {
		for key := range nativeTLS {
			if key == "tls" {
				continue
			}
			if _, exists := node[key]; exists {
				return "unsupported_tls_combination", nil
			}
		}
	}
	if oneOf(kind, "ss", "vless", "anytls") {
		if _, present := node["udp"]; !present {
			node["udp"] = true
		}
	}
	return "", nil
}
