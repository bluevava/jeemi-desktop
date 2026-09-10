package subscriptionformat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func standardURI(kind string, fields map[string]any) string {
	query := url.Values{}
	for key, value := range fields {
		switch value.(type) {
		case []any, []string, map[string]any:
			encoded, _ := json.Marshal(value)
			query.Set(key, string(encoded))
		default:
			query.Set(key, fmt.Sprint(value))
		}
	}
	return kind + "://?" + query.Encode()
}

func TestStandardMihomoURIProtocolCatalog(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	cases := map[string]map[string]any{
		"http":        {"tls": true, "sni": "tls.example.invalid", "headers": map[string]any{"User-Agent": "fixture"}},
		"socks5":      {"udp": true},
		"ss":          {"cipher": "aes-128-gcm", "password": "fixture", "plugin": "obfs", "plugin-opts": map[string]any{"mode": "tls", "host": "tls.example.invalid"}},
		"ssr":         {"cipher": "chacha20-ietf", "password": "fixture", "protocol": "auth_sha1_v4", "obfs": "tls1.2_ticket_auth"},
		"snell":       {"psk": "fixture", "version": 5, "obfs-opts": map[string]any{"mode": "http", "host": "edge.example.invalid"}},
		"vmess":       {"uuid": testUUID, "alterId": 0, "cipher": "auto", "tls": true, "network": "ws", "ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "cdn.example.invalid"}, "max-early-data": 2048}},
		"vless":       {"uuid": testUUID, "tls": true, "servername": "tls.example.invalid", "network": "grpc", "grpc-opts": map[string]any{"grpc-service-name": "fixture", "max-connections": 2}},
		"trojan":      {"password": "fixture", "network": "ws", "ws-opts": map[string]any{"path": "/ws"}},
		"anytls":      {"password": "fixture", "client-fingerprint": "firefox", "tfo": false, "alpn": []string{"h2", "http/1.1"}},
		"mieru":       {"username": "fixture", "password": "fixture", "transport": "TCP", "multiplexing": "MULTIPLEXING_LOW"},
		"sudoku":      {"key": testUUID, "aead-method": "chacha20-poly1305", "httpmask": map[string]any{"disable": false, "mode": "stream", "tls": true}},
		"hysteria":    {"auth-str": "fixture", "protocol": "udp", "up": "30 Mbps", "down": "200 Mbps"},
		"hysteria2":   {"password": "fixture", "ports": "443-8443", "hop-interval": "15-30", "obfs": "salamander", "obfs-password": "fixture"},
		"tuic":        {"uuid": testUUID, "password": "fixture", "heartbeat-interval": 10000, "reduce-rtt": false},
		"shadowquic":  {"username": "fixture", "password": "fixture", "quic-versions": []string{"v1", "v2"}},
		"wireguard":   {"private-key": key, "public-key": key, "ip": "10.0.0.2", "allowed-ips": []string{"0.0.0.0/0"}, "reserved": []any{1, 2, 3}, "amnezia-wg-option": map[string]any{"version": 2, "jc": 5}},
		"tailscale":   {"hostname": "fixture", "auth-key": "fixture", "ephemeral": false, "exit-node": "100.64.0.1"},
		"ssh":         {"username": "fixture", "private-key": "-----BEGIN PRIVATE KEY-----\nfixture\n-----END PRIVATE KEY-----", "host-key": []string{"ssh-ed25519 fixture"}},
		"masque":      {"private-key": key, "public-key": key, "network": "h2", "ip": "10.0.0.2/32", "ip-stack": map[string]any{"mode": "auto"}},
		"trusttunnel": {"username": "fixture", "password": "fixture", "health-check": true, "quic": false},
		"zerotier":    {"network": "0123456789abcdef", "secondary-port": -1, "orbit": []any{map[string]any{"world": "0123456789abcdef", "seed": "0123456789"}}},
		"openvpn":     {"username": "fixture", "password": "fixture", "ca": "-----BEGIN CERTIFICATE-----\nfixture\n-----END CERTIFICATE-----", "proto": "udp", "data-ciphers": []string{"AES-256-GCM", "AES-128-GCM"}},
		"direct":      {"udp": true}, "dns": {}, "rematch": {"target-rematch-name": "fixture"},
	}
	if len(cases) != len(nativeProtocols) {
		t.Fatal("every registered protocol needs a native URI contract fixture")
	}
	for kind, fields := range cases {
		t.Run(kind, func(t *testing.T) {
			fields["name"], fields["type"] = "🇭🇰 Fixture "+kind, kind
			if !nativeProtocols[kind].noServer {
				fields["server"], fields["port"] = "node.example.invalid", 443
			}
			raw := []byte(standardURI(kind, fields))
			before := bytes.Clone(raw)
			result, err := Normalize(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(raw, before) || result.Report.ProxyCount != 1 || result.Report.SkippedNodes != 0 {
				t.Fatal("source mutation or node loss")
			}
			got := proxy(t, result, 0)
			for key, value := range fields {
				wantJSON, _ := json.Marshal(value)
				gotJSON, _ := json.Marshal(got[key])
				if !bytes.Equal(wantJSON, gotJSON) {
					t.Errorf("field %s changed shape or value", key)
				}
			}
		})
	}
}

func TestMixedURISkipsMalformedNodesAndPreservesMetadata(t *testing.T) {
	good := standardURI("trojan", map[string]any{"name": "Good", "server": "node.entry.example.invalid", "port": 443, "password": "fixture", "tfo": false})
	bad := []string{"future-protocol://pw@node.example.invalid:443", "not a URI", "trojan://pw@node.example.invalid:bad", "trojan://?server=node.example.invalid&port=443", "trojan://pw@node.example.invalid:443?udp=maybe", "trojan://pw@node.example.invalid:443?secret-unknown=value", "anytls://pw@node.example.invalid:443?tfo=0&tfo=true"}
	text := "host://node.entry.example.invalid?server=192.0.2.53\n" + strings.Join(bad, "\n") + "\n" + good
	for _, raw := range []string{text, base64.StdEncoding.EncodeToString([]byte(text))} {
		r := normalize(t, raw)
		if r.Report.ProxyCount != 1 || r.Report.SkippedNodes != len(bad) || len(r.Report.Diagnostics) != len(bad) || proxy(t, r, 0)["name"] != "Good" {
			t.Fatal("bad node blocked or altered good nodes")
		}
		if len(configuration(t, r)["dns"].(map[string]any)["proxy-server-nameserver-policy"].(map[string]any)) != 1 {
			t.Fatal("custom node DNS lost")
		}
		for _, diagnostic := range r.Report.Diagnostics {
			if diagnostic.Line < 2 || diagnostic.Line > len(bad)+1 || strings.Contains(fmt.Sprint(diagnostic), "secret-unknown") {
				t.Fatal("diagnostic leaked input or lost line number")
			}
		}
	}
	if _, err := Normalize([]byte("host://node.entry.example.invalid?server=invalid\n" + good)); err == nil {
		t.Fatal("invalid shared DNS was silently discarded")
	}
	if _, err := Normalize([]byte(strings.Join(bad, "\n"))); err == nil {
		t.Fatal("zero-node input succeeded")
	}
}

func TestNativeURIConflictAndNestedTypeValidation(t *testing.T) {
	base := "trojan://fixture@node.example.invalid:443?"
	for _, query := range []string{
		"password=other", "server=other.example.invalid", "port=8443", "fp=firefox&client-fingerprint=chrome", "sni=one.example.invalid&peer=two.example.invalid",
		"network=ws&ws-opts=" + url.QueryEscape(`{"path":"/one"}`) + "&path=%2Ftwo",
		"network=ws&ws-opts=" + url.QueryEscape(`{"path":"/one","path":"/two"}`),
		"network=ws&ws-opts=" + url.QueryEscape(`[]`), "alpn=" + url.QueryEscape(`[1,2]`), "udp=%FF",
		"security=reality", "tls=false", "network=tcp&ws-opts=" + url.QueryEscape(`{"path":"/ws"}`),
	} {
		r := normalize(t, base+query+"\n"+simpleURI)
		if r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 1 {
			t.Fatal("conflicting or malformed native field accepted")
		}
	}
	equivalent := base + "network=ws&type=ws&ws-opts=" + url.QueryEscape(`{"path":"/ws","headers":{"Host":"cdn.example.invalid"}}`) + "&path=%2Fws&host=cdn.example.invalid"
	if r := normalize(t, equivalent); r.Report.ProxyCount != 1 {
		t.Fatal("equivalent nested alias rejected")
	}
}

func TestNativeURIKeepsLegacyEncodingsAndAliases(t *testing.T) {
	vmess := `{"v":"2","ps":"VMess fixture","add":"node.example.invalid","port":"443","id":"` + testUUID + `","aid":"0","scy":"auto","net":"ws","type":"none","host":"cdn.example.invalid","path":"/ws","tls":"tls","sni":"tls.example.invalid"}`
	ssr := "node.example.invalid:443:auth_sha1_v4:aes-128-gcm:tls1.2_ticket_auth:" + base64.RawURLEncoding.EncodeToString([]byte("fixture")) + "/?remarks=" + base64.RawURLEncoding.EncodeToString([]byte("SSR fixture"))
	for scheme, body := range map[string]string{"vmess": vmess, "ssr": ssr} {
		r := normalize(t, scheme+"://"+base64.RawURLEncoding.EncodeToString([]byte(body)))
		if proxy(t, r, 0)["type"] != scheme || r.Report.ProxyCount != 1 {
			t.Fatal("legacy envelope failed")
		}
	}
	for alias, kind := range map[string]string{"https": "http", "hy2": "hysteria2", "socks": "socks5", "s5": "socks5"} {
		r := normalize(t, alias+"://fixture:fixture@node.example.invalid:443")
		if proxy(t, r, 0)["type"] != kind {
			t.Fatal("scheme alias failed")
		}
	}
	uri := "tuic://" + testUUID + ":fixture@node.example.invalid:443?heartbeat=10s&heartbeat-interval=10000&congestion_control=bbr&congestion-controller=bbr"
	if proxy(t, normalize(t, uri), 0)["heartbeat-interval"] != 10000 {
		t.Fatal("duration alias conflict or unit loss")
	}
	canonical := standardURI("anytls", map[string]any{"server": "node.example.invalid", "port": 443, "password": "fixture", "tfo": false, "idle-session-timeout": 30})
	encoded := base64.StdEncoding.EncodeToString([]byte(canonical))
	if !reflect.DeepEqual(proxy(t, normalize(t, canonical), 0), proxy(t, normalize(t, encoded), 0)) {
		t.Fatal("base64 and plain native URI differ")
	}
}

func TestNativeURIListRecoveryAndCompatibilityBoundaries(t *testing.T) {
	good := "anytls://fixture@node.example.invalid:443?alpn=" + url.QueryEscape(`["h2","http/1.1"]`) + "&idle-session-timeout=30s&interface-name=fixture"
	r := normalize(t, good)
	got := proxy(t, r, 0)
	if !reflect.DeepEqual(got["alpn"], []any{"h2", "http/1.1"}) || got["idle-session-timeout"] != 30 {
		t.Fatal("native arrays or legacy duration units changed")
	}
	legacyKeys := "anytls://fixture@node.example.invalid:443?alpn=" + url.QueryEscape(`["h2","http/1.1"]`)
	if got := proxy(t, normalize(t, legacyKeys), 0); !reflect.DeepEqual(got["alpn"], []any{"h2", "http/1.1"}) {
		t.Fatal("legacy parsing split the canonical ALPN JSON")
	}
	for _, prefix := range []string{"broken first URI", "future://" + strings.Repeat("x", maxLineBytes+1)} {
		mixed := prefix + "\n" + good
		for _, raw := range []string{mixed, base64.StdEncoding.EncodeToString([]byte(mixed))} {
			result := normalize(t, raw)
			if result.Report.ProxyCount != 1 || result.Report.SkippedNodes != 1 {
				t.Fatal("bad first node or oversized node blocked the subscription")
			}
		}
	}
	native := "description: |\n  trojan://this-is-only-text\nproxies: []\n"
	if result := normalize(t, native); result.Report.Format != "mihomo" || string(result.Contents) != native {
		t.Fatal("URI inside YAML reclassified the native document")
	}
	ss := normalize(t, "ss://node.example.invalid:8388?cipher=aes-128-gcm&password=fixture#Query")
	if proxy(t, ss, 0)["password"] != "fixture" {
		t.Fatal("native authority with query credentials rejected")
	}
}

func TestNativeWireGuardMultiPeerShape(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	fields := map[string]any{"private-key": key, "refresh-server-ip-interval": 300, "peers": []any{
		map[string]any{"server": "one.example.invalid", "port": 51820, "public-key": key, "allowed-ips": []any{"0.0.0.0/0"}, "reserved": []any{1, 2, 3}},
		map[string]any{"server": "two.example.invalid", "port": 51821, "public-key": key, "allowed-ips": []any{"::/0"}},
	}}
	result := normalize(t, standardURI("wireguard", fields))
	if !reflect.DeepEqual(proxy(t, result, 0)["peers"], fields["peers"]) {
		t.Fatal("multi-peer node lost fields or required a top-level endpoint")
	}
	fields["peers"].([]any)[0].(map[string]any)["port"] = "invalid"
	bad := standardURI("wireguard", fields) + "\n" + simpleURI
	if r := normalize(t, bad); r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 1 {
		t.Fatal("invalid peer endpoint was not skipped")
	}
}

func TestNativeTLSFieldsStayWithinProtocolCapabilities(t *testing.T) {
	for _, raw := range []string{
		"http://node.example.invalid:443?tls=true&client-fingerprint=chrome",
		"http://node.example.invalid:443?tls=true&security=reality",
		"socks5://node.example.invalid:443?name-cert-verify=tls.example.invalid",
		"trojan://fixture@node.example.invalid:443?tlsmirror-opts=" + url.QueryEscape(`{"primary-key":"fixture"}`),
		"anytls://fixture@node.example.invalid:443?certificate=fixture",
	} {
		r := normalize(t, raw+"\n"+simpleURI)
		if r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 1 {
			t.Fatal("incompatible native TLS fields were silently accepted")
		}
	}
}
