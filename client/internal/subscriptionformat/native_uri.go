package subscriptionformat

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func parseProxyURI(raw string, line int) (proxyNode, string, error) {
	if u, err := url.Parse(raw); err == nil {
		for _, alpn := range u.Query()["alpn"] {
			if strings.HasPrefix(strings.TrimSpace(alpn), "[") {
				return parseNativeProxyURI(raw, line)
			}
		}
	}
	// Keep the existing, strictly validated dialects and their exact defaults.
	if node, reason, err := parseLegacyProxyURI(raw, line); err == nil && reason == "" {
		return node, "", nil
	}
	return parseNativeProxyURI(raw, line)
}

func parseNativeProxyURI(raw string, line int) (proxyNode, string, error) {
	n := proxyNode{Line: line}
	scheme, _, _ := strings.Cut(raw, ":")
	scheme = strings.ToLower(scheme)
	n.Protocol = canonicalProtocol(scheme)
	definition, known := nativeProtocols[n.Protocol]
	if !known {
		return n, "unsupported_protocol", nil
	}
	var err error
	raw, err = expandCompatibleURI(raw, n.Protocol, line)
	if err != nil {
		return n, "", err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return n, "", failure("invalid_uri", line, "")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return n, "", failure("invalid_uri", line, "")
	}
	values := map[string]any{"type": n.Protocol}
	if u.Host != "" {
		server, err := normalizeServer(u.Hostname())
		if err != nil {
			return n, "", failure("invalid_field", line, "server")
		}
		values["server"] = server
		if u.Port() != "" {
			port, err := strconv.Atoi(u.Port())
			if err != nil {
				return n, "", failure("invalid_field", line, "port")
			}
			values["port"] = port
		}
	}
	if err := nativeAuthority(values, u.User, n.Protocol, line); err != nil {
		return n, "", err
	}
	if u.Fragment != "" {
		values["name"] = u.Fragment
	}
	if scheme == "https" {
		values["tls"] = true
	}
	// Canonical fields are processed first. Alias insertion below uses the same
	// conflict checks and cannot override any canonical field or URI authority.
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	aliases := url.Values{}
	for _, key := range keys {
		kind, recognized := definition.fields[key]
		if key == "type" && query.Get(key) != n.Protocol {
			recognized = false // Existing type=ws/tcp is a transport alias.
		}
		if !recognized {
			aliases[key] = query[key]
			continue
		}
		for _, text := range query[key] {
			value, ok := parseNativeValue(text, kind)
			// Both historical spellings also accepted duration strings.
			if n.Protocol == "anytls" && oneOf(key, "idle-session-check-interval", "idle-session-timeout") {
				seconds, valid := normalizeField(text, "seconds")
				ok = valid
				if valid {
					value = int(seconds.(int64))
				}
			}
			if !ok {
				return n, "", failure("invalid_field", line, "node")
			}
			target := key
			switch key {
			case "server", "sni", "servername":
				if text != "" {
					value, err = normalizeServer(text)
					if err != nil {
						return n, "", failure("invalid_field", line, "server")
					}
				}
				if key != "server" {
					target = nativeSNIKey(n.Protocol)
				}
			case "fingerprint":
				value, ok = normalizeField(text, "pin")
				if !ok {
					return n, "", failure("invalid_field", line, "certificate_pin")
				}
			}
			if !mergeNativeField(values, target, value) {
				return n, "", failure("conflicting_alias", line, "node")
			}
		}
	}
	if reason, err := applyNativeAliases(values, aliases, n.Protocol, line); err != nil || reason != "" {
		return n, reason, err
	}
	if reason, err := validateNativeProxy(values, n.Protocol, definition, line); err != nil || reason != "" {
		return n, reason, err
	}
	n.Name, n.Server = nativeText(values, "name"), nativeText(values, "server")
	n.Port, _ = values["port"].(int)
	n.TLS.Enabled = definition.tlsAlways || nativeFlag(values, "tls")
	n.TLS.ServerName = nativeText(values, nativeSNIKey(n.Protocol))
	n.TLS.LeafSHA256 = nativeText(values, "fingerprint")
	if reality, ok := values["reality-opts"].(map[string]any); ok {
		n.TLS.RealityPublicKey = nativeText(reality, "public-key")
	}
	n.Transport.Network = nativeText(values, "network")
	n.Encryption = nativeText(values, "encryption")
	n.Native = values
	return n, "", nil
}

func nativeSNIKey(protocol string) string {
	if protocol == "vmess" || protocol == "vless" {
		return "servername"
	}
	return "sni"
}

func nativeAuthority(values map[string]any, user *url.Userinfo, protocol string, line int) error {
	if user == nil {
		return nil
	}
	username := user.Username()
	password, paired := user.Password()
	switch protocol {
	case "ss", "ssr":
		if !paired {
			decoded, ok := decodeBase64(username)
			if !ok {
				return failure("invalid_field", line, "credentials")
			}
			username, password, paired = strings.Cut(string(decoded), ":")
		}
		if !paired {
			return failure("invalid_field", line, "credentials")
		}
		values["cipher"], values["password"] = username, password
	case "vmess", "vless":
		if paired {
			return failure("invalid_field", line, "credentials")
		}
		values["uuid"] = username
	case "tuic":
		if paired {
			values["uuid"], values["password"] = username, password
		} else {
			values["token"] = username
		}
	case "anytls", "trojan", "hysteria2", "snell", "hysteria", "sudoku":
		if paired {
			if protocol != "hysteria2" {
				return failure("invalid_field", line, "credentials")
			}
			username += ":" + password
		}
		key := "password"
		switch protocol {
		case "snell":
			key = "psk"
		case "hysteria":
			key = "auth-str"
		case "sudoku":
			key = "key"
		}
		values[key] = username
	case "http", "socks5", "ssh", "mieru", "shadowquic", "trusttunnel":
		values["username"] = username
		if paired {
			values["password"] = password
		}
	default:
		return failure("invalid_field", line, "credentials")
	}
	return nil
}
