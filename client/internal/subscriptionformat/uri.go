package subscriptionformat

import (
	"encoding/hex"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

func parseURIList(text string) (parsedDocument, error) {
	p := parsedDocument{}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		return p, failure("size_limit", 0, "")
	}
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > maxLineBytes {
			return p, failure("size_limit", index+1, "")
		}
		if !uriStart.MatchString(line) {
			return p, failure("invalid_uri", index+1, "")
		}
		scheme, _, _ := strings.Cut(line, ":")
		switch strings.ToLower(scheme) {
		case "host":
			policy, err := parseHostURI(line, index+1)
			if err != nil {
				return p, err
			}
			p.Policies = append(p.Policies, policy)
		case "cert":
			cert, err := parseCertificateURI(line, index+1)
			if err != nil {
				return p, err
			}
			p.Certificates = append(p.Certificates, cert)
		default:
			if len(line) > 32<<10 {
				return p, failure("size_limit", index+1, "")
			}
			node, unsupported, err := parseProxyURI(line, index+1)
			if err != nil {
				return p, err
			}
			if unsupported != "" {
				p.skip(unsupported, index+1, "")
				continue
			}
			p.Nodes = append(p.Nodes, node)
		}
	}
	return p, nil
}

func parseProxyURI(raw string, line int) (proxyNode, string, error) {
	n := proxyNode{Line: line}
	scheme, _, _ := strings.Cut(raw, ":")
	switch strings.ToLower(scheme) {
	case "ss", "shadowsocks":
		n.Protocol = "ss"
	case "socks", "socks5", "s5":
		n.Protocol = "socks5"
	case "anytls", "tuic", "vless", "snell":
		n.Protocol = strings.ToLower(scheme)
	default:
		return n, "unsupported_protocol", nil
	}
	// Legacy SS wraps the entire authority, rather than only userinfo.
	if n.Protocol == "ss" {
		prefix, fragment, hasFragment := strings.Cut(raw, "#")
		_, authority, _ := strings.Cut(prefix, "://")
		if !strings.Contains(authority, "@") {
			decoded, ok := decodeBase64(authority)
			if !ok {
				return n, "", failure("invalid_uri", line, "")
			}
			at := strings.LastIndexByte(string(decoded), '@')
			if at < 0 {
				return n, "", failure("invalid_uri", line, "")
			}
			cipher, password, valid := strings.Cut(string(decoded[:at]), ":")
			if !valid || !validCredential(password) {
				return n, "", failure("invalid_field", line, "credentials")
			}
			// Full-authority SS base64 contains literal credentials. Escape them
			// for URL parsing once; percent signs here are not URI escapes.
			raw = "ss://" + url.UserPassword(cipher, password).String() + "@" + string(decoded[at+1:])
			if hasFragment {
				raw += "#" + fragment
			}
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Host == "" || u.User == nil || (u.Path != "" && u.Path != "/") {
		return n, "", failure("invalid_uri", line, "")
	}
	n.Server, err = normalizeServer(u.Hostname())
	if err != nil {
		return n, "", failure("invalid_field", line, "server")
	}
	n.Port, err = strconv.Atoi(u.Port())
	if err != nil || n.Port < 1 || n.Port > 65535 {
		return n, "", failure("invalid_field", line, "port")
	}
	n.Name = u.Fragment
	if !validText(n.Name, 1024) {
		return n, "", failure("invalid_field", line, "name")
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return n, "", failure("invalid_uri", line, "")
	}
	r := reader(values, line)
	password, hasPassword := u.User.Password()
	switch n.Protocol {
	case "ss":
		n.Cipher = u.User.Username()
		n.Password = password
		if !hasPassword {
			b, ok := decodeBase64(n.Cipher)
			if !ok {
				return n, "", failure("invalid_field", line, "credentials")
			}
			n.Cipher, n.Password, hasPassword = strings.Cut(string(b), ":")
		}
		if !hasPassword || !validCredential(n.Password) || n.Cipher == "" {
			return n, "", failure("invalid_field", line, "credentials")
		}
		n.UDP = r.boolean("udp")
		if n.UDP == nil {
			n.UDP = boolValue(true)
		}
	case "socks5":
		n.Username = u.User.Username()
		n.Password = password
		if !hasPassword || !validCredential(n.Username) || !validCredential(n.Password) {
			return n, "", failure("invalid_field", line, "credentials")
		}
		n.UDP = r.boolean("udp")
	case "anytls":
		if hasPassword {
			return n, "", failure("invalid_field", line, "credentials")
		}
		n.Password = u.User.Username()
		if !validCredential(n.Password) {
			return n, "", failure("invalid_field", line, "credentials")
		}
		n.TLS = readTLS(r)
		n.TLS.Enabled = true
		n.UDP = r.boolean("udp")
		if n.UDP == nil {
			n.UDP = boolValue(true)
		}
		n.IdleCheck = r.number("idle_check")
		n.IdleTimeout = r.number("idle_timeout")
		n.MinIdle = r.number("min_idle")
	case "tuic":
		n.UUID = u.User.Username()
		n.Password = password
		if !validUUID(n.UUID) || !hasPassword || !validCredential(n.Password) {
			return n, "", failure("invalid_field", line, "credentials")
		}
		n.TLS = readTLS(r)
		n.TLS.Enabled = true
		security := r.text("security")
		if security != "" && security != "tls" {
			return n, "unsupported_transport", r.err
		}
		n.UDP = r.boolean("udp")
		n.Congestion = r.text("congestion")
		n.UDPRelayMode = r.text("udp_relay_mode")
		n.ReduceRTT = r.boolean("reduce_rtt")
		n.Heartbeat = r.number("heartbeat")
		if !oneOf(n.Congestion, "", "cubic", "new_reno", "bbr") || !oneOf(n.UDPRelayMode, "", "native", "quic") {
			r.err = failure("invalid_field", line, "tuic_options")
		}
	case "vless":
		n.UUID = u.User.Username()
		if hasPassword || !validUUID(n.UUID) {
			return n, "", failure("invalid_field", line, "credentials")
		}
		if reason := readVLESS(r, &n); reason != "" {
			return n, reason, r.err
		}
	case "snell":
		n.PSK = u.User.Username()
		if hasPassword || !validCredential(n.PSK) {
			return n, "", failure("invalid_field", line, "credentials")
		}
		if psk := r.text("psk"); psk != "" && psk != n.PSK {
			r.err = failure("conflicting_alias", line, "psk")
		}
		version := r.number("version")
		n.SnellVersion = 1
		if version != nil {
			n.SnellVersion = int(*version)
		}
		userkey := r.text("userkey")
		mode := r.text("mode")
		n.ObfsMode = r.text("obfs")
		n.ObfsHost = r.text("obfs_host")
		n.UDP = r.boolean("udp")
		if r.err != nil {
			return n, "", r.err
		}
		if userkey != "" {
			return n, "snell_userkey_unsupported", nil
		}
		if n.SnellVersion < 1 || n.SnellVersion > 5 || mode != "" {
			return n, "unsupported_protocol_version", nil
		}
		if !oneOf(n.ObfsMode, "", "none", "http", "tls") {
			return n, "unsupported_transport", nil
		}
	}
	if r.err != nil {
		return n, "", r.err
	}
	if r.unknown() {
		return n, "unsupported_node_fields", nil
	}
	if reason := unsupportedNodeFeature(n); reason != "" {
		return n, reason, nil
	}
	if err := validateNode(n); err != nil {
		return n, "", err
	}
	return n, "", nil
}

func readTLS(r *fieldReader) tlsOptions {
	return tlsOptions{ServerName: r.text("sni"), LeafSHA256: r.text("certificate_pin"), ClientFingerprint: r.text("client_fingerprint"), ALPN: r.list("alpn"), SkipVerify: r.boolean("skip_cert_verify")}
}

func unsupportedNodeFeature(n proxyNode) string {
	// Official TUIC has neither a ClientHello fingerprint option nor a UDP
	// disable switch. Accept its intrinsic UDP=true without emitting a fake key.
	if n.Protocol == "tuic" {
		if n.TLS.ClientFingerprint != "" {
			return "unsupported_node_fields"
		}
		if n.UDP != nil && !*n.UDP {
			return "unsupported_udp_setting"
		}
	}
	return ""
}

func readVLESS(r *fieldReader, n *proxyNode) string {
	n.TLS = readTLS(r)
	security := r.text("security")
	if !oneOf(security, "", "none", "tls", "reality") {
		return "unsupported_transport"
	}
	n.TLS.Enabled = security == "tls" || security == "reality"
	n.Transport = transportOptions{Network: r.text("network"), Path: r.text("path"), Host: r.text("host"), ServiceName: r.text("service_name"), Mode: r.text("mode")}
	if n.Transport.Network == "" {
		n.Transport.Network = "tcp"
	}
	if !oneOf(n.Transport.Network, "tcp", "ws", "grpc", "xhttp") {
		return "unsupported_transport"
	}
	n.Flow = r.text("flow")
	n.Encryption = r.text("encryption")
	n.PacketEncoding = r.text("packet_encoding")
	if n.Encryption == "none" {
		n.Encryption = ""
	}
	n.UDP = r.boolean("udp")
	if n.UDP == nil {
		n.UDP = boolValue(true)
	}
	key, id := r.text("reality_public_key"), r.text("reality_short_id")
	if security == "reality" {
		b, ok := decodeBase64(key)
		short, e := hex.DecodeString(id)
		if !ok || len(b) != 32 || e != nil || len(short) > 8 || n.TLS.ServerName == "" {
			r.err = failure("invalid_field", r.line, "reality")
			return ""
		}
		n.TLS.RealityPublicKey = key
		n.TLS.RealityShortID = id
		if n.TLS.LeafSHA256 != "" || n.TLS.SkipVerify != nil {
			return "unsupported_tls_combination"
		}
	} else if key != "" || id != "" {
		r.err = failure("invalid_field", r.line, "reality")
	}
	if !n.TLS.Enabled && (n.TLS.LeafSHA256 != "" || n.TLS.SkipVerify != nil || n.TLS.ServerName != "" || n.TLS.ClientFingerprint != "" || len(n.TLS.ALPN) > 0) {
		return "unsupported_tls_combination"
	}
	if !oneOf(n.PacketEncoding, "", "xudp", "packetaddr", "none") || !oneOf(n.Flow, "", "xtls-rprx-vision") {
		return "unsupported_transport"
	}
	switch n.Transport.Network {
	case "ws":
		if n.Transport.ServiceName != "" || n.Transport.Mode != "" {
			return "unsupported_transport"
		}
	case "grpc":
		if n.Transport.ServiceName == "" {
			r.err = failure("invalid_field", r.line, "service_name")
		}
		if n.Transport.Path != "" || n.Transport.Host != "" || n.Transport.Mode != "" {
			return "unsupported_transport"
		}
	case "tcp":
		if n.Transport.Path != "" || n.Transport.Host != "" || n.Transport.ServiceName != "" || n.Transport.Mode != "" {
			return "unsupported_transport"
		}
	case "xhttp":
		if !oneOf(n.Transport.Mode, "", "auto", "packet-up", "stream-up", "stream-one") || n.Transport.ServiceName != "" {
			return "unsupported_transport"
		}
	}
	if n.Transport.Path != "" && !strings.HasPrefix(n.Transport.Path, "/") {
		r.err = failure("invalid_field", r.line, "path")
	}
	return ""
}

func normalizeServer(value string) (string, error) {
	if ip := net.ParseIP(value); ip != nil {
		return ip.String(), nil
	}
	v, err := idna.Lookup.ToASCII(strings.TrimSuffix(value, "."))
	if err != nil || v == "" || len(v) > 253 || strings.ContainsAny(v, "/:@?#*+ \\[]") {
		return "", failure("invalid_field", 0, "server")
	}
	for _, label := range strings.Split(v, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", failure("invalid_field", 0, "server")
		}
	}
	return strings.ToLower(v), nil
}
func validText(value string, max int) bool {
	return len(value) <= max && utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsControl(r) })
}
func validCredential(value string) bool { return value != "" && validText(value, maxFieldBytes) }
func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	b, e := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return e == nil && len(b) == 16
}
func oneOf(value string, values ...string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func validateNode(n proxyNode) error {
	if n.TLS.ServerName != "" {
		if _, err := normalizeServer(n.TLS.ServerName); err != nil {
			return failure("invalid_field", n.Line, "sni")
		}
	}
	if n.UDP != nil && *n.UDP && n.Protocol == "snell" && n.SnellVersion < 3 {
		return failure("invalid_field", n.Line, "udp")
	}
	return nil
}
