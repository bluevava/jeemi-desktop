package subscriptionformat

// This catalog describes configuration fields, not proxy implementations.
// Names/types follow https://wiki.metacubex.one/config/proxies/ (2026-09-10).
// Nested objects retain native JSON structure; the selected external core still
// validates protocol-specific options before applying a runtime configuration.
var nativeCommon = combineFields(
	fieldNames(nativeString, "name type server ip-version interface-name dialer-proxy"),
	fieldNames(nativeInteger, "port routing-mark"),
	fieldNames(nativeBool, "udp tfo mptcp"),
	fieldNames(nativeObject, "smux"),
)

var nativeTLS = combineFields(
	fieldNames(nativeString, "sni servername fingerprint client-fingerprint name-cert-verify"),
	fieldNames(nativeMultiline, "certificate private-key"),
	fieldNames(nativeBool, "tls skip-cert-verify"),
	fieldNames(nativeStrings, "alpn"),
	fieldNames(nativeObject, "ech-opts shadow-tls-opts restls-opts jls-opts tlsmirror-opts"),
)

var nativeTransport = combineFields(
	fieldNames(nativeString, "network"),
	fieldNames(nativeObject, "http-opts h2-opts grpc-opts ws-opts xhttp-opts mkcp-opts mekya-opts reality-opts"),
)

var nativeIPStack = combineFields(
	fieldNames(nativeObject, "ip-stack"),
	fieldNames(nativeString, "ip ipv6"),
	fieldNames(nativeInteger, "mtu"),
	fieldNames(nativeBool, "remote-dns-resolve"),
	fieldNames(nativeStrings, "dns"),
)

type nativeProtocol struct {
	fields    nativeFields
	required  string
	noServer  bool
	tlsAlways bool
	networks  string
}

func protocol(required string, sets ...nativeFields) nativeProtocol {
	return nativeProtocol{fields: combineFields(append([]nativeFields{nativeCommon}, sets...)...), required: required}
}

var nativeProtocols = func() map[string]nativeProtocol {
	text, multi, integer, boolean, object, list := nativeString, nativeMultiline, nativeInteger, nativeBool, nativeObject, nativeStrings
	result := map[string]nativeProtocol{
		"http":        protocol("", nativeTLS, fieldNames(text, "username password"), fieldNames(object, "headers")),
		"socks5":      protocol("", nativeTLS, fieldNames(text, "username password")),
		"ss":          protocol("cipher password", fieldNames(text, "cipher password plugin client-fingerprint"), fieldNames(object, "plugin-opts"), fieldNames(boolean, "udp-over-tcp"), fieldNames(integer, "udp-over-tcp-version")),
		"ssr":         protocol("cipher password obfs protocol", fieldNames(text, "cipher password obfs protocol obfs-param protocol-param")),
		"snell":       protocol("psk", fieldNames(text, "psk"), fieldNames(integer, "version"), fieldNames(object, "obfs-opts")),
		"vmess":       protocol("uuid", nativeTLS, nativeTransport, fieldNames(text, "uuid cipher packet-encoding"), fieldNames(integer, "alterId"), fieldNames(boolean, "global-padding authenticated-length")),
		"vless":       protocol("uuid", nativeTLS, nativeTransport, fieldNames(text, "uuid flow encryption packet-encoding")),
		"trojan":      protocol("password", nativeTLS, nativeTransport, fieldNames(text, "password"), fieldNames(object, "ss-opts")),
		"anytls":      protocol("password", nativeTLS, fieldNames(text, "password client-metadata"), fieldNames(integer, "idle-session-check-interval idle-session-timeout min-idle-session")),
		"mieru":       protocol("username password", fieldNames(text, "username password port-range transport multiplexing handshake-mode traffic-pattern")),
		"sudoku":      protocol("key", fieldNames(text, "key aead-method table-type custom-table multiplex"), fieldNames(list, "custom-tables"), fieldNames(integer, "padding-min padding-max"), fieldNames(boolean, "enable-pure-downlink"), fieldNames(object, "httpmask")),
		"hysteria":    protocol("", nativeTLS, fieldNames(text, "auth auth-str obfs protocol up down ports"), fieldNames(integer, "recv-window-conn recv-window"), fieldNames(boolean, "disable_mtu_discovery fast-open")),
		"hysteria2":   protocol("password", nativeTLS, fieldNames(text, "ports hop-interval password up down bbr-profile obfs obfs-password"), fieldNames(object, "realm-opts"), fieldNames(integer, "obfs-min-packet-size obfs-max-packet-size handshake-timeout initial-stream-receive-window max-stream-receive-window initial-connection-receive-window max-connection-receive-window")),
		"tuic":        protocol("", nativeTLS, fieldNames(text, "token uuid password ip udp-relay-mode congestion-controller bbr-profile"), fieldNames(integer, "heartbeat-interval request-timeout max-udp-relay-packet-size max-open-streams"), fieldNames(boolean, "disable-sni reduce-rtt fast-open")),
		"shadowquic":  protocol("username password", nativeTLS, fieldNames(text, "username password congestion-controller up down bbr-profile"), fieldNames(list, "quic-versions"), fieldNames(integer, "keep-alive-interval cwnd max-datagram-frame-size max-open-streams recv-window-conn recv-window"), fieldNames(boolean, "udp-over-stream zero-rtt disable-mtu-discovery")),
		"wireguard":   protocol("private-key", nativeIPStack, fieldNames(text, "private-key public-key pre-shared-key"), fieldNames(integer, "workers persistent-keepalive refresh-server-ip-interval"), fieldNames(nativeArray, "peers"), fieldNames(nativeReserved, "reserved"), fieldNames(list, "allowed-ips"), fieldNames(object, "amnezia-wg-option")),
		"tailscale":   protocol("", fieldNames(text, "hostname auth-key control-url state-dir exit-node"), fieldNames(boolean, "ephemeral accept-routes exit-node-allow-lan-access")),
		"ssh":         protocol("username", fieldNames(text, "username password private-key-passphrase"), fieldNames(multi, "private-key"), fieldNames(list, "host-key host-key-algorithms")),
		"masque":      protocol("private-key public-key", nativeTLS, nativeIPStack, fieldNames(text, "public-key network congestion-controller bbr-profile"), fieldNames(integer, "handshake-timeout")),
		"trusttunnel": protocol("username password", nativeTLS, fieldNames(text, "username password congestion-controller bbr-profile"), fieldNames(boolean, "health-check quic"), fieldNames(integer, "max-connections min-streams max-streams")),
		"zerotier":    protocol("network", nativeIPStack, fieldNames(text, "network state-dir planet tcp-fallback-mode tcp-fallback-relay remote-trace-target"), fieldNames(integer, "physical-mtu primary-port remote-trace-level"), fieldNames(nativeSignedInteger, "secondary-port"), fieldNames(boolean, "low-bandwidth encrypted-hello"), fieldNames(nativeArray, "orbit")),
		"openvpn":     protocol("ca", nativeIPStack, fieldNames(text, "proto username password key-direction dev cipher data-ciphers-fallback auth comp-lzo"), fieldNames(multi, "ca cert key tls-auth tls-crypt tls-crypt-v2"), fieldNames(list, "data-ciphers"), fieldNames(integer, "ping ping-restart tran-window handshake-timeout"), fieldNames(object, "peer-info")),
		"direct":      protocol(""),
		"dns":         protocol(""),
		"rematch":     protocol("", fieldNames(text, "target-rematch-name target-sub-rule")),
	}
	for _, name := range []string{"tailscale", "zerotier", "direct", "dns", "rematch"} {
		p := result[name]
		p.noServer = true
		result[name] = p
	}
	for _, name := range []string{"anytls", "trojan", "hysteria", "hysteria2", "tuic", "shadowquic", "masque", "trusttunnel"} {
		p := result[name]
		p.tlsAlways = true
		result[name] = p
	}
	for name, networks := range map[string]string{"vmess": "tcp ws http h2 grpc mkcp mekya", "vless": "tcp ws http h2 grpc xhttp", "trojan": "tcp ws grpc", "masque": "h3 h2 h3-l4proxy"} {
		p := result[name]
		p.networks = networks
		result[name] = p
	}
	return result
}()

func canonicalProtocol(scheme string) string {
	switch scheme {
	case "shadowsocks":
		return "ss"
	case "shadowsocksr":
		return "ssr"
	case "socks", "s5":
		return "socks5"
	case "https":
		return "http"
	case "hy2":
		return "hysteria2"
	case "wg":
		return "wireguard"
	}
	return scheme
}
