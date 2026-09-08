package schema

import "jeemi/internal/config/compose"

const Version = "mihomo-2026.09-v1"

type ValueKind string

const (
	KindScalar        ValueKind = "scalar"
	KindMapping       ValueKind = "mapping"
	KindSequence      ValueKind = "sequence"
	KindNamedMapping  ValueKind = "named_mapping"
	KindNamedSequence ValueKind = "named_sequence"
	KindRules         ValueKind = "rules"
)

type EditorKind string

const (
	EditorBoolean EditorKind = "boolean"
	EditorNumber  EditorKind = "number"
	EditorString  EditorKind = "string"
	EditorEnum    EditorKind = "enum"
	EditorYAML    EditorKind = "yaml"
)

type FieldScope string

const (
	ScopeDirect     FieldScope = ""
	ScopeAllProxies FieldScope = "all_proxies"
)

type Field struct {
	ID               string                 `json:"id"`
	Path             string                 `json:"path"`
	Kind             ValueKind              `json:"kind"`
	Editor           EditorKind             `json:"editor"`
	Options          []string               `json:"options"`
	ExampleYAML      string                 `json:"exampleYaml"`
	Strategies       []compose.Strategy     `json:"strategies"`
	DefaultStrategy  compose.Strategy       `json:"defaultStrategy"`
	DefaultConflict  compose.ConflictPolicy `json:"defaultConflictPolicy"`
	Locked           bool                   `json:"locked"`
	Sensitive        bool                   `json:"sensitive"`
	DocumentationURL string                 `json:"documentationUrl"`
	Hidden           bool                   `json:"hidden"`
	Scope            FieldScope             `json:"scope"`
	TargetPath       string                 `json:"targetPath"`
	ApplicableTypes  []string               `json:"applicableTypes"`
}

type Category struct {
	ID               string  `json:"id"`
	DocumentationURL string  `json:"documentationUrl"`
	Fields           []Field `json:"fields"`
}

type Catalog struct {
	Version    string     `json:"version"`
	Categories []Category `json:"categories"`
}

const docsBase = "https://wiki.metacubex.one/config"

var catalog = Catalog{
	Version: Version,
	Categories: []Category{
		{
			ID:               "general",
			DocumentationURL: docsBase + "/general/",
			Fields: []Field{
				lockedField("allow-lan", "/allow-lan", KindScalar),
				lockedField("bind-address", "/bind-address", KindScalar),
				sequenceField("lan-allowed-ips", "/lan-allowed-ips", "- 0.0.0.0/0\n- ::/0"),
				sequenceField("lan-disallowed-ips", "/lan-disallowed-ips", "- 192.168.0.0/16"),
				sensitiveSequenceField("authentication", "/authentication", `- "username:password"`),
				sequenceField("skip-auth-prefixes", "/skip-auth-prefixes", "- 127.0.0.1/8\n- ::1/128"),
				lockedField("mode", "/mode", KindScalar),
				lockedField("log-level", "/log-level", KindScalar),
				lockedField("ipv6", "/ipv6", KindScalar),
				numberField("keep-alive-interval", "/keep-alive-interval", "15"),
				numberField("keep-alive-idle", "/keep-alive-idle", "15"),
				boolField("disable-keep-alive", "/disable-keep-alive", "false"),
				lockedField("find-process-mode", "/find-process-mode", KindScalar),
				mappingField("profile", "/profile", "store-selected: true\nstore-fake-ip: true"),
				boolField("unified-delay", "/unified-delay", "true"),
				boolField("tcp-concurrent", "/tcp-concurrent", "true"),
				stringField("interface-name", "/interface-name", "Ethernet"),
				numberField("routing-mark", "/routing-mark", "6666"),
				mappingField("tls", "/tls", "certificate: \"\"\nprivate-key: \"\""),
				lockedField("geodata-mode", "/geodata-mode", KindScalar),
				lockedField("geodata-loader", "/geodata-loader", KindScalar),
				lockedField("geo-auto-update", "/geo-auto-update", KindScalar),
				lockedField("geo-update-interval", "/geo-update-interval", KindScalar),
				lockedField("geox-url", "/geox-url", KindMapping),
				stringField("global-ua", "/global-ua", "mihomo"),
				boolField("etag-support", "/etag-support", "true"),
			},
		},
		{
			ID:               "controller",
			DocumentationURL: docsBase + "/general/#_7",
			Fields: []Field{
				lockedField("external-controller", "/external-controller", KindScalar),
				lockedField("secret", "/secret", KindScalar),
				lockedField("external-controller-cors", "/external-controller-cors", KindMapping),
				lockedField("external-controller-unix", "/external-controller-unix", KindScalar),
				lockedField("external-controller-pipe", "/external-controller-pipe", KindScalar),
				lockedField("external-controller-tls", "/external-controller-tls", KindScalar),
				lockedField("external-controller-routing-mark", "/external-controller-routing-mark", KindScalar),
				lockedField("external-doh-server", "/external-doh-server", KindScalar),
			},
		},
		{
			ID:               "inbound",
			DocumentationURL: docsBase + "/inbound/port/",
			Fields: []Field{
				lockedField("port", "/port", KindScalar),
				lockedField("socks-port", "/socks-port", KindScalar),
				lockedField("mixed-port", "/mixed-port", KindScalar),
				numberField("redir-port", "/redir-port", "7892"),
				numberField("tproxy-port", "/tproxy-port", "7893"),
				namedSequenceField("listeners", "/listeners", "- name: local-http\n  type: http\n  port: 7890\n  listen: 127.0.0.1"),
			},
		},
		{
			ID:               "dns",
			DocumentationURL: docsBase + "/dns/",
			Fields: []Field{
				lockedField("dns-enable", "/dns/enable", KindScalar),
				lockedField("dns-listen", "/dns/listen", KindScalar),
				lockedField("dns-ipv6", "/dns/ipv6", KindScalar),
				numberField("dns-ipv6-timeout", "/dns/ipv6-timeout", "100"),
				lockedField("dns-use-hosts", "/dns/use-hosts", KindScalar),
				boolField("dns-use-system-hosts", "/dns/use-system-hosts", "true"),
				boolField("dns-respect-rules", "/dns/respect-rules", "false"),
				enumField("dns-cache-algorithm", "/dns/cache-algorithm", "arc", "lru", "arc"),
				boolField("dns-prefer-h3", "/dns/prefer-h3", "false"),
				sequenceField("dns-default-nameserver", "/dns/default-nameserver", "- 223.5.5.5\n- 1.1.1.1"),
				lockedField("dns-nameserver", "/dns/nameserver", KindSequence),
				sequenceField("dns-fallback", "/dns/fallback", "- tls://1.1.1.1"),
				lockedField("dns-proxy-server-nameserver", "/dns/proxy-server-nameserver", KindSequence),
				sequenceField("dns-direct-nameserver", "/dns/direct-nameserver", "- system"),
				boolField("dns-direct-follow-policy", "/dns/direct-nameserver-follow-policy", "false"),
				lockedField("dns-nameserver-policy", "/dns/nameserver-policy", KindMapping),
				lockedField("dns-proxy-server-nameserver-policy", "/dns/proxy-server-nameserver-policy", KindMapping),
				lockedField("dns-enhanced-mode", "/dns/enhanced-mode", KindScalar),
				lockedField("dns-fake-ip-range", "/dns/fake-ip-range", KindScalar),
				lockedField("dns-fake-ip-range6", "/dns/fake-ip-range6", KindScalar),
				lockedField("dns-fake-ip-filter-mode", "/dns/fake-ip-filter-mode", KindScalar),
				lockedField("dns-fake-ip-filter", "/dns/fake-ip-filter", KindSequence),
				mappingField("dns-fallback-filter", "/dns/fallback-filter", "geoip: true\ngeoip-code: CN"),
			},
		},
		{
			ID:               "sniffer",
			DocumentationURL: docsBase + "/sniff/",
			Fields: []Field{
				documented(boolField("sniffer-enable", "/sniffer/enable", "true"), docsBase+"/sniff/#enable"),
				documented(boolField("sniffer-force-dns-mapping", "/sniffer/force-dns-mapping", "true"), docsBase+"/sniff/#force-dns-mapping"),
				documented(boolField("sniffer-parse-pure-ip", "/sniffer/parse-pure-ip", "true"), docsBase+"/sniff/#parse-pure-ip"),
				documented(boolField("sniffer-override-destination", "/sniffer/override-destination", "false"), docsBase+"/sniff/#override-destination"),
				documented(sequenceField("sniffer-http-ports", "/sniffer/sniff/HTTP/ports", "- 80\n- 8080-8880"), docsBase+"/sniff/#sniff"),
				documented(boolField("sniffer-http-override-destination", "/sniffer/sniff/HTTP/override-destination", "true"), docsBase+"/sniff/#sniff"),
				documented(sequenceField("sniffer-tls-ports", "/sniffer/sniff/TLS/ports", "- 443\n- 8443"), docsBase+"/sniff/#sniff"),
				documented(boolField("sniffer-tls-override-destination", "/sniffer/sniff/TLS/override-destination", "false"), docsBase+"/sniff/#sniff"),
				documented(sequenceField("sniffer-quic-ports", "/sniffer/sniff/QUIC/ports", "- 443\n- 8443"), docsBase+"/sniff/#sniff"),
				documented(boolField("sniffer-quic-override-destination", "/sniffer/sniff/QUIC/override-destination", "false"), docsBase+"/sniff/#sniff"),
				documented(sequenceField("sniffer-force-domain", "/sniffer/force-domain", "- +.v2ex.com"), docsBase+"/sniff/#force-domain"),
				documented(sequenceField("sniffer-skip-domain", "/sniffer/skip-domain", "- Mijia Cloud"), docsBase+"/sniff/#skip-domain"),
				documented(sequenceField("sniffer-skip-src-address", "/sniffer/skip-src-address", "- 192.168.0.3/32"), docsBase+"/sniff/#skip-src-address"),
				documented(sequenceField("sniffer-skip-dst-address", "/sniffer/skip-dst-address", "- 192.168.0.3/32"), docsBase+"/sniff/#skip-dst-address"),
			},
		},
		{
			ID:               "tun",
			DocumentationURL: docsBase + "/inbound/tun/",
			Fields: []Field{
				lockedField("tun-enable", "/tun/enable", KindScalar),
				lockedField("tun-stack", "/tun/stack", KindScalar),
				sequenceField("tun-dns-hijack", "/tun/dns-hijack", "- any:53\n- tcp://any:53"),
				lockedField("tun-auto-route", "/tun/auto-route", KindScalar),
				boolField("tun-auto-redirect", "/tun/auto-redirect", "false"),
				lockedField("tun-auto-detect-interface", "/tun/auto-detect-interface", KindScalar),
				boolField("tun-strict-route", "/tun/strict-route", "true"),
				boolField("tun-endpoint-independent-nat", "/tun/endpoint-independent-nat", "false"),
				numberField("tun-mtu", "/tun/mtu", "9000"),
				lockedField("tun-device", "/tun/device", KindScalar),
				numberField("tun-udp-timeout", "/tun/udp-timeout", "300"),
				boolField("tun-gso", "/tun/gso", "false"),
				numberField("tun-gso-max-size", "/tun/gso-max-size", "65536"),
				sequenceField("tun-route-address", "/tun/route-address", "- 0.0.0.0/1\n- 128.0.0.0/1"),
				sequenceField("tun-route-address-set", "/tun/route-address-set", "- cncidr"),
				lockedField("tun-route-exclude-address", "/tun/route-exclude-address", KindSequence),
				sequenceField("tun-route-exclude-address-set", "/tun/route-exclude-address-set", "- private"),
				sequenceField("tun-include-interface", "/tun/include-interface", "- Ethernet"),
				sequenceField("tun-exclude-interface", "/tun/exclude-interface", "- Ethernet 2"),
			},
		},
		{
			ID:               "outbound",
			DocumentationURL: docsBase + "/proxies/",
			Fields: []Field{
				documented(proxyOverrideEnumField("proxies-ip-version", "/proxies/*/ip-version", "/ip-version", "dual", nil, "dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer"), docsBase+"/proxies/#ip-version"),
				documented(proxyOverrideBoolField("proxies-udp", "/proxies/*/udp", "/udp", "true", nil), docsBase+"/proxies/#udp"),
				documented(proxyOverrideStringField("proxies-interface-name", "/proxies/*/interface-name", "/interface-name", "Ethernet", nil), docsBase+"/proxies/#interface-name"),
				documented(proxyOverrideNumberField("proxies-routing-mark", "/proxies/*/routing-mark", "/routing-mark", "6666", nil), docsBase+"/proxies/#routing-mark"),
				documented(proxyOverrideBoolField("proxies-tfo", "/proxies/*/tfo", "/tfo", "false", nil), docsBase+"/proxies/#tfo"),
				documented(proxyOverrideBoolField("proxies-mptcp", "/proxies/*/mptcp", "/mptcp", "false", nil), docsBase+"/proxies/#mptcp"),
				documented(proxyOverrideStringField("proxies-dialer-proxy", "/proxies/*/dialer-proxy", "/dialer-proxy", "DIRECT", nil), docsBase+"/proxies/dialer-proxy/"),
				documented(proxyOverrideEnumField("proxies-client-fingerprint", "/proxies/*/client-fingerprint", "/client-fingerprint", "chrome", []string{"vmess", "vless", "trojan", "anytls"}, "chrome", "firefox", "safari", "ios", "android", "edge", "360", "qq", "random"), docsBase+"/proxies/tls/#client-fingerprint"),
				namedMappingField("proxy-providers", "/proxy-providers", "provider-name:\n  type: http\n  url: https://example.invalid/subscription\n  path: ./proxy-providers/provider.yaml"),
			},
		},
		{
			ID:               "routing",
			DocumentationURL: docsBase + "/rules/",
			Fields: []Field{
				namedMappingField("sub-rules", "/sub-rules", "local-sub-rule:\n  - DOMAIN-SUFFIX,example.com,DIRECT"),
			},
		},
		{
			ID:               "hosts",
			DocumentationURL: docsBase + "/dns/hosts/",
			Fields: []Field{
				lockedField("hosts", "/hosts", KindMapping),
			},
		},
		{
			ID:               "advanced",
			DocumentationURL: docsBase + "/",
			Fields: []Field{
				sequenceField("tunnels", "/tunnels", "- network: [tcp]\n  address: 127.0.0.1:8080\n  target: example.com:80\n  proxy: DIRECT"),
				mappingField("ntp", "/ntp", "enable: false\nserver: time.apple.com\nport: 123\ninterval: 30"),
				mappingField("experimental", "/experimental", "quic-go-disable-gso: false"),
			},
		},
	},
}

func Current() Catalog {
	return catalog
}

func FindField(path string) (Field, bool) {
	for _, category := range catalog.Categories {
		for _, field := range category.Fields {
			if field.Path == path {
				return field, true
			}
		}
	}
	return Field{}, false
}

func boolField(id, path, example string) Field {
	return scalarField(id, path, EditorBoolean, example)
}

func numberField(id, path, example string) Field {
	return scalarField(id, path, EditorNumber, example)
}

func stringField(id, path, example string) Field {
	return scalarField(id, path, EditorString, example)
}

func enumField(id, path, example string, options ...string) Field {
	field := scalarField(id, path, EditorEnum, example)
	field.Options = options
	return field
}

func scalarField(id, path string, editor EditorKind, example string) Field {
	return Field{
		ID:              id,
		Path:            path,
		Kind:            KindScalar,
		Editor:          editor,
		Options:         []string{},
		ExampleYAML:     example,
		Strategies:      []compose.Strategy{compose.StrategyReplace},
		DefaultStrategy: compose.StrategyReplace,
	}
}

func mappingField(id, path, example string) Field {
	return Field{
		ID:              id,
		Path:            path,
		Kind:            KindMapping,
		Editor:          EditorYAML,
		Options:         []string{},
		ExampleYAML:     example,
		Strategies:      []compose.Strategy{compose.StrategyMerge, compose.StrategyReplace},
		DefaultStrategy: compose.StrategyMerge,
	}
}

func sequenceField(id, path, example string) Field {
	return Field{
		ID:              id,
		Path:            path,
		Kind:            KindSequence,
		Editor:          EditorYAML,
		Options:         []string{},
		ExampleYAML:     example,
		Strategies:      []compose.Strategy{compose.StrategyPrepend, compose.StrategyAppend, compose.StrategyReplace},
		DefaultStrategy: compose.StrategyAppend,
	}
}

func sensitiveSequenceField(id, path, example string) Field {
	field := sequenceField(id, path, example)
	field.Sensitive = true
	return field
}

func namedMappingField(id, path, example string) Field {
	return Field{
		ID:              id,
		Path:            path,
		Kind:            KindNamedMapping,
		Editor:          EditorYAML,
		Options:         []string{},
		ExampleYAML:     example,
		Strategies:      []compose.Strategy{compose.StrategyMergeByName, compose.StrategyReplace},
		DefaultStrategy: compose.StrategyMergeByName,
		DefaultConflict: compose.ConflictError,
	}
}

func namedSequenceField(id, path, example string) Field {
	return Field{
		ID:              id,
		Path:            path,
		Kind:            KindNamedSequence,
		Editor:          EditorYAML,
		Options:         []string{},
		ExampleYAML:     example,
		Strategies:      []compose.Strategy{compose.StrategyMergeByName, compose.StrategyPrepend, compose.StrategyAppend, compose.StrategyReplace},
		DefaultStrategy: compose.StrategyMergeByName,
		DefaultConflict: compose.ConflictError,
	}
}

func documented(field Field, url string) Field {
	field.DocumentationURL = url
	return field
}

func proxyOverrideBoolField(id, path, targetPath, example string, applicableTypes []string) Field {
	return proxyOverrideField(scalarField(id, path, EditorBoolean, example), targetPath, applicableTypes)
}

func proxyOverrideNumberField(id, path, targetPath, example string, applicableTypes []string) Field {
	return proxyOverrideField(scalarField(id, path, EditorNumber, example), targetPath, applicableTypes)
}

func proxyOverrideStringField(id, path, targetPath, example string, applicableTypes []string) Field {
	return proxyOverrideField(scalarField(id, path, EditorString, example), targetPath, applicableTypes)
}

func proxyOverrideEnumField(id, path, targetPath, example string, applicableTypes []string, options ...string) Field {
	return proxyOverrideField(enumField(id, path, example, options...), targetPath, applicableTypes)
}

func proxyOverrideField(field Field, targetPath string, applicableTypes []string) Field {
	field.Scope = ScopeAllProxies
	field.TargetPath = targetPath
	field.ApplicableTypes = append([]string{}, applicableTypes...)
	return field
}

func lockedField(id, path string, kind ValueKind) Field {
	return Field{
		ID: id, Path: path, Kind: kind, Editor: EditorYAML,
		Options: []string{}, Strategies: []compose.Strategy{}, Locked: true,
	}
}
