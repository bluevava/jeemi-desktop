package subscriptionformat

import (
	"bytes"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"
)

func TestAnyTLSURICompatibilityOptions(t *testing.T) {
	const base = "anytls://fixture-password@192.0.2.10:443?fp=firefox&sni=tls.example.invalid&udp=1"
	for _, tc := range []struct {
		name, query string
		tfo         any
	}{
		{"implicit_transport", "", nil},
		{"explicit_tcp", "&type=tcp", nil},
		{"explicit_tls", "&security=tls", nil},
		{"tfo_disabled", "&tfo=0", false},
		{"shared_uri", "&type=tcp&security=tls&tfo=0", false},
		{"tfo_enabled", "&type=tcp&security=tls&tfo=1", true},
		{"equivalent_aliases", "&type=tcp&network=tcp&security=tls&tfo=false&tfo=0", false},
		{"network_alias", "&network=tcp&security=tls&tfo=true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uri := base + tc.query + "#HK Example 04"
			for encoding, text := range map[string]string{
				"plain":  uri,
				"base64": base64.StdEncoding.EncodeToString([]byte(uri)),
			} {
				t.Run(encoding, func(t *testing.T) {
					raw := []byte(text)
					before := bytes.Clone(raw)
					r, err := Normalize(raw)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(raw, before) || r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 0 || r.Report.Encoding != encoding {
						t.Fatal("source, node count or URI envelope changed")
					}
					n := proxy(t, r, 0)
					for key, want := range map[string]any{
						"name": "HK Example 04", "type": "anytls", "server": "192.0.2.10", "port": 443,
						"password": "fixture-password", "sni": "tls.example.invalid", "client-fingerprint": "firefox", "udp": true,
					} {
						if !reflect.DeepEqual(n[key], want) {
							t.Fatalf("wrong AnyTLS field %s", key)
						}
					}
					value, present := n["tfo"]
					if present != (tc.tfo != nil) || !reflect.DeepEqual(value, tc.tfo) {
						t.Fatal("TCP Fast Open value or explicit false was lost")
					}
					for _, key := range []string{"network", "security"} {
						if _, exists := n[key]; exists {
							t.Fatalf("URI transport marker leaked into AnyTLS YAML: %s", key)
						}
					}
				})
			}
		})
	}
}

func TestAnyTLSURIRejectsIncompatibleTransportAndOptions(t *testing.T) {
	const base = "anytls://fixture-password@node.example.invalid:443?"
	for _, tc := range []struct {
		name, query, code, skipped string
	}{
		{"websocket", "type=ws", "no_supported_nodes", "unsupported_transport"},
		{"quic", "network=quic", "no_supported_nodes", "unsupported_transport"},
		{"plaintext", "security=none", "no_supported_nodes", "unsupported_transport"},
		{"reality", "security=reality", "no_supported_nodes", "unsupported_transport"},
		{"conflicting_network", "type=tcp&network=ws", "conflicting_alias", ""},
		{"conflicting_security", "security=tls&security=none", "conflicting_alias", ""},
		{"conflicting_tfo", "tfo=0&tfo=true", "conflicting_alias", ""},
		{"invalid_tfo", "tfo=enabled", "invalid_field", ""},
		{"empty_tfo", "tfo=", "invalid_field", ""},
		{"unknown_extension", "type=tcp&security=tls&tfo=0&unrecognized=value", "no_supported_nodes", "unsupported_node_fields"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := base + tc.query
			_, err := Normalize([]byte(raw))
			var detail *Error
			if !errors.As(err, &detail) || detail.Code != "no_supported_nodes" {
				t.Fatalf("unexpected rejection: %v", err)
			}
			want := tc.skipped
			if want == "" {
				want = tc.code
			}
			r := normalize(t, simpleURI+"\n"+raw)
			if r.Report.ProxyCount != 1 || r.Report.SkippedNodes != 1 || len(r.Report.Diagnostics) != 1 || r.Report.Diagnostics[0].Code != want {
				t.Fatal("incompatible AnyTLS was accepted or silently downgraded")
			}
		})
	}
}
