package subscriptionformat

import (
	"net/url"
	"strings"
)

// Existing aliases remain semantic conversions, never arbitrary passthrough.
func applyNativeAliases(node map[string]any, values url.Values, protocol string, line int) (string, error) {
	r := reader(values, line)
	set := func(key string, value any) {
		if !mergeNativeField(node, key, value) {
			r.err = failure("conflicting_alias", line, "node")
		}
	}
	alias := func(id, key string) {
		if value := r.read(id); value != nil {
			if n, ok := value.(int64); ok {
				value = int(n)
			}
			if _, allowed := nativeProtocols[protocol].fields[key]; !allowed {
				r.err = failure("invalid_field", line, "node")
				return
			}
			set(key, value)
		}
	}
	alias("sni", nativeSNIKey(protocol))
	alias("client_fingerprint", "client-fingerprint")
	alias("certificate_pin", "fingerprint")
	alias("skip_cert_verify", "skip-cert-verify")
	alias("udp", "udp")
	if protocol == "anytls" {
		alias("idle_check", "idle-session-check-interval")
		alias("idle_timeout", "idle-session-timeout")
		alias("min_idle", "min-idle-session")
	}
	if protocol == "tuic" {
		alias("congestion", "congestion-controller")
		alias("udp_relay_mode", "udp-relay-mode")
		alias("reduce_rtt", "reduce-rtt")
		alias("heartbeat", "heartbeat-interval")
	}
	if protocol == "vless" || protocol == "vmess" {
		alias("packet_encoding", "packet-encoding")
	}
	security := r.text("security")
	if security != "" {
		if security == "reality" && !oneOf(protocol, "vmess", "vless", "trojan") {
			return "unsupported_transport", r.err
		}
		definition := nativeProtocols[protocol]
		if definition.tlsAlways {
			if security != "tls" && !(security == "reality" && protocol == "trojan") {
				return "unsupported_transport", r.err
			}
		} else if _, allowed := definition.fields["tls"]; !allowed || !oneOf(security, "none", "tls", "reality") {
			return "unsupported_transport", r.err
		}
		if !definition.tlsAlways {
			set("tls", security != "none")
		}
	}
	network := r.text("network")
	if network != "" {
		if protocol == "anytls" {
			if network != "tcp" {
				return "unsupported_transport", r.err
			}
		} else if nativeProtocols[protocol].networks != "" {
			set("network", network)
		} else {
			return "unsupported_transport", r.err
		}
	}
	if oneOf(protocol, "vless", "vmess", "trojan") {
		key, short := r.text("reality_public_key"), r.text("reality_short_id")
		if key != "" || short != "" {
			if security != "reality" {
				return "", failure("invalid_field", line, "reality")
			}
			set("reality-opts", map[string]any{"public-key": key, "short-id": short})
		}
		if security == "reality" {
			if _, present := node["reality-opts"]; !present {
				return "", failure("invalid_field", line, "reality")
			}
		}
		path, host, service, mode := r.text("path"), r.text("host"), r.text("service_name"), r.text("mode")
		transport := nativeText(node, "network")
		if path != "" && !strings.HasPrefix(path, "/") {
			return "", failure("invalid_field", line, "path")
		}
		opts := map[string]any{}
		switch transport {
		case "ws":
			if service != "" || mode != "" {
				return "unsupported_transport", r.err
			}
			if path != "" {
				opts["path"] = path
			}
			if host != "" {
				opts["headers"] = map[string]any{"Host": host}
			}
		case "grpc":
			if path != "" || host != "" || mode != "" {
				return "unsupported_transport", r.err
			}
			if service != "" {
				opts["grpc-service-name"] = service
			}
		case "xhttp":
			if service != "" || !oneOf(mode, "", "auto", "packet-up", "stream-up", "stream-one") {
				return "unsupported_transport", r.err
			}
			if path != "" {
				opts["path"] = path
			}
			if host != "" {
				opts["host"] = host
			}
			if mode != "" {
				opts["mode"] = mode
			}
			if path != "" || host != "" || mode != "" {
				// Keep the established URI alias default when no native
				// option explicitly chooses another value.
				native, _ := node["xhttp-opts"].(map[string]any)
				if _, present := native["no-grpc-header"]; !present {
					opts["no-grpc-header"] = false
				}
			}
		case "http":
			if service != "" || mode != "" {
				return "unsupported_transport", r.err
			}
			if path != "" {
				opts["path"] = []any{path}
			}
			if host != "" {
				opts["headers"] = map[string]any{"Host": []any{host}}
			}
		case "h2":
			if service != "" || mode != "" {
				return "unsupported_transport", r.err
			}
			if path != "" {
				opts["path"] = path
			}
			if host != "" {
				opts["host"] = []any{host}
			}
		default:
			if path != "" || host != "" || service != "" || mode != "" {
				return "unsupported_transport", r.err
			}
		}
		if len(opts) != 0 {
			set(transport+"-opts", opts)
		}
	}
	if protocol == "snell" {
		mode, host, userkey := r.text("obfs"), r.text("obfs_host"), r.text("userkey")
		if userkey != "" {
			return "snell_userkey_unsupported", r.err
		}
		if !oneOf(mode, "", "none", "http", "tls") {
			return "unsupported_transport", r.err
		}
		if host != "" && (mode == "" || mode == "none") {
			return "unsupported_transport", r.err
		}
		if mode != "" && mode != "none" {
			opts := map[string]any{"mode": mode}
			if host != "" {
				opts["host"] = host
			}
			set("obfs-opts", opts)
		}
	}
	if r.err != nil {
		return "", r.err
	}
	if r.unknown() {
		return "unsupported_node_fields", nil
	}
	return "", nil
}
