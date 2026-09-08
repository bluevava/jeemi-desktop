package runtimeconfig

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDefaultPreferencesAreValid(t *testing.T) {
	preferences := DefaultPreferences()
	if err := Validate(preferences); err != nil {
		t.Fatalf("Validate(DefaultPreferences()) error = %v", err)
	}
	if preferences.OutboundMode != OutboundModeRule ||
		preferences.ProxyMode != ProxyModeTUN ||
		preferences.ListenerType != ListenerTypeMixed ||
		preferences.ListenPort != DefaultListenPort ||
		preferences.AllowLAN ||
		preferences.TUNStack != TUNStackMixed ||
		preferences.LogLevel != LogLevelSilent ||
		preferences.FindProcessMode != FindProcessModeStrict ||
		!preferences.IPv6 ||
		!preferences.DNSEnabled ||
		preferences.DNSListen != DefaultDNSListen ||
		preferences.DNSIPv6 ||
		preferences.DNSUseHosts ||
		preferences.DNSEnhancedMode != DNSEnhancedModeFakeIP ||
		preferences.DNSFakeIPRange != DefaultFakeIPRange ||
		preferences.DNSFakeIPRange6 != DefaultFakeIPRange6 ||
		!preferences.TUNAutoRoute ||
		!preferences.TUNAutoDetectInterface ||
		preferences.TUNRouteExcludeAddress == nil ||
		len(preferences.TUNRouteExcludeAddress) != 0 ||
		preferences.TUNRouteExcludeAddressEnabled ||
		preferences.TUNRouteExcludeAddressMerge != MergeModeAppend ||
		!preferences.DNSNameserverEnabled ||
		!preferences.DNSFakeIPFilterEnabled ||
		preferences.DNSProxyServerNameserverEnabled ||
		preferences.DNSNameserverPolicyEnabled ||
		preferences.DNSProxyServerNameserverPolicyEnabled ||
		preferences.DNSNameserverMerge != MergeModeAppend ||
		preferences.DNSFakeIPFilterMerge != MergeModeOverride ||
		!reflect.DeepEqual(preferences.LANBypassRules, DefaultLANBypassRules()) {
		t.Fatalf("unexpected defaults: %+v", preferences)
	}
}

func TestTUNDeviceNameIsStableExceptForMacOS(t *testing.T) {
	if got := TUNDeviceNameForOS("windows"); got != "JeemiTun" {
		t.Fatalf("windows TUN device = %q", got)
	}
	if got := TUNDeviceNameForOS("linux"); got != "JeemiTun" {
		t.Fatalf("linux TUN device = %q", got)
	}
	if got := TUNDeviceNameForOS("darwin"); got != "" {
		t.Fatalf("macOS TUN device must be assigned by the OS, got %q", got)
	}
	if got := TUNDeviceNameForOS("freebsd"); got != "" {
		t.Fatalf("unsupported platform TUN device = %q", got)
	}
}

func TestValidateRejectsEachInvalidRuntimePreference(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Preferences)
	}{
		{name: "outbound mode", mutate: func(value *Preferences) { value.OutboundMode = "invalid" }},
		{name: "proxy mode", mutate: func(value *Preferences) { value.ProxyMode = "invalid" }},
		{name: "listener type", mutate: func(value *Preferences) { value.ListenerType = "invalid" }},
		{name: "listen port", mutate: func(value *Preferences) { value.ListenPort = 0 }},
		{name: "TUN stack", mutate: func(value *Preferences) { value.TUNStack = "invalid" }},
		{name: "log level", mutate: func(value *Preferences) { value.LogLevel = "verbose" }},
		{name: "find process mode", mutate: func(value *Preferences) { value.FindProcessMode = "auto" }},
		{name: "empty find process mode", mutate: func(value *Preferences) { value.FindProcessMode = "" }},
		{name: "DNS listen", mutate: func(value *Preferences) { value.DNSListen = "0.0.0.0" }},
		{name: "DNS listen conflicts with proxy", mutate: func(value *Preferences) { value.DNSListen = "127.0.0.1:7890" }},
		{name: "DNS enhanced mode", mutate: func(value *Preferences) { value.DNSEnhancedMode = "normal" }},
		{name: "DNS fake IP range", mutate: func(value *Preferences) { value.DNSFakeIPRange = "10.0.0.1" }},
		{name: "DNS fake IPv6 range", mutate: func(value *Preferences) { value.DNSFakeIPRange6 = "198.18.0.1/16" }},
		{name: "TUN route exclude address", mutate: func(value *Preferences) {
			value.TUNRouteExcludeAddressEnabled = true
			value.TUNRouteExcludeAddress = []string{"192.168.1.1"}
		}},
		{name: "TUN route exclude merge", mutate: func(value *Preferences) { value.TUNRouteExcludeAddressMerge = "prepend" }},
		{name: "DNS nameservers", mutate: func(value *Preferences) { value.DNSNameservers = []string{} }},
		{name: "DNS nameserver merge", mutate: func(value *Preferences) { value.DNSNameserverMerge = "prepend" }},
		{name: "DNS policy YAML", mutate: func(value *Preferences) {
			value.DNSNameserverPolicyEnabled = true
			value.DNSNameserverPolicyYAML = "- not-a-map"
		}},
		{name: "hosts YAML", mutate: func(value *Preferences) {
			value.DNSUseHosts = true
			value.HostsYAML = "host: {nested: invalid}"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preferences := DefaultPreferences()
			test.mutate(&preferences)
			if err := Validate(preferences); err == nil {
				t.Fatalf("Validate() accepted %+v", preferences)
			}
		})
	}
}

func TestNormalizeRecoversMissingAndInvalidStoredValues(t *testing.T) {
	normalized := Normalize(Preferences{OutboundMode: "unknown", ListenPort: 70000, AllowLAN: true})
	defaults := DefaultPreferences()
	if normalized.OutboundMode != defaults.OutboundMode ||
		normalized.ProxyMode != defaults.ProxyMode ||
		normalized.ListenerType != defaults.ListenerType ||
		normalized.ListenPort != defaults.ListenPort ||
		normalized.TUNStack != defaults.TUNStack ||
		normalized.FindProcessMode != FindProcessModeStrict ||
		!normalized.AllowLAN {
		t.Fatalf("Normalize() = %+v", normalized)
	}
}

func TestPreferencesJSONDefaultsOmittedFieldsWithoutOverwritingExplicitFalse(t *testing.T) {
	var preferences Preferences
	if err := json.Unmarshal([]byte(`{
  "outboundMode": "global",
  "proxyMode": "tun",
  "listenerType": "http",
  "listenPort": 8080,
  "allowLan": true,
  "tunStack": "system",
  "ipv6": false,
  "tunAutoRoute": false
}`), &preferences); err != nil {
		t.Fatal(err)
	}
	if preferences.IPv6 || preferences.TUNAutoRoute {
		t.Fatalf("explicit false values were overwritten: %+v", preferences)
	}
	if preferences.LogLevel != LogLevelSilent || preferences.FindProcessMode != FindProcessModeStrict || !preferences.DNSEnabled || preferences.DNSListen != DefaultDNSListen {
		t.Fatalf("missing fields did not receive defaults: %+v", preferences)
	}
	if !reflect.DeepEqual(preferences.DNSNameservers, defaultDNSNameservers) {
		t.Fatalf("missing DNS nameservers = %#v", preferences.DNSNameservers)
	}
	if !preferences.DNSNameserverEnabled || !preferences.DNSFakeIPFilterEnabled ||
		preferences.DNSProxyServerNameserverEnabled || preferences.DNSNameserverPolicyEnabled ||
		preferences.DNSProxyServerNameserverPolicyEnabled {
		t.Fatalf("missing DNS resource switches did not receive defaults: %+v", preferences)
	}
	if !reflect.DeepEqual(preferences.LANBypassRules, DefaultLANBypassRules()) {
		t.Fatalf("missing LAN bypass rules did not receive defaults: %#v", preferences.LANBypassRules)
	}
	if preferences.TUNRouteExcludeAddress == nil || len(preferences.TUNRouteExcludeAddress) != 0 {
		t.Fatalf("missing TUN route exclusions did not receive an empty default: %#v", preferences.TUNRouteExcludeAddress)
	}
	if preferences.TUNRouteExcludeAddressEnabled || preferences.TUNRouteExcludeAddressMerge != MergeModeAppend {
		t.Fatalf("missing TUN route exclusion controls did not receive defaults: %+v", preferences)
	}
}

func TestPreferencesJSONPreservesExplicitNonSilentLogLevel(t *testing.T) {
	var preferences Preferences
	if err := json.Unmarshal([]byte(`{"logLevel":"warning"}`), &preferences); err != nil {
		t.Fatal(err)
	}
	if preferences.LogLevel != LogLevelWarning {
		t.Fatalf("explicit log level was replaced: %+v", preferences)
	}
}

func TestPreferencesJSONDoesNotEnableOptionalDNSFromValues(t *testing.T) {
	var preferences Preferences
	if err := json.Unmarshal([]byte(`{
  "dnsProxyServerNameservers": ["1.1.1.1"],
  "dnsNameserverPolicyYaml": "'+.example.test': 8.8.8.8",
  "dnsProxyServerNameserverPolicyMerge": "override"
}`), &preferences); err != nil {
		t.Fatal(err)
	}
	if preferences.DNSProxyServerNameserverEnabled || preferences.DNSNameserverPolicyEnabled || preferences.DNSProxyServerNameserverPolicyEnabled {
		t.Fatalf("optional DNS values implicitly enabled injection: %+v", preferences)
	}
}

func TestPreferencesJSONDoesNotEnableTUNRouteExclusionFromValues(t *testing.T) {
	var preferences Preferences
	if err := json.Unmarshal([]byte(`{
  "tunRouteExcludeAddress": ["192.168.50.0/24"]
}`), &preferences); err != nil {
		t.Fatal(err)
	}
	if preferences.TUNRouteExcludeAddressEnabled || preferences.TUNRouteExcludeAddressMerge != MergeModeAppend {
		t.Fatalf("TUN route exclusion values implicitly enabled injection: %+v", preferences)
	}

	var minimal Preferences
	if err := json.Unmarshal([]byte(`{"outboundMode":"rule"}`), &minimal); err != nil {
		t.Fatal(err)
	}
	if minimal.TUNRouteExcludeAddressEnabled || minimal.TUNRouteExcludeAddressMerge != MergeModeAppend {
		t.Fatalf("omitted route exclusions did not retain defaults: %+v", minimal)
	}
}

func TestRuntimeYAMLMappingsAcceptOnlyStringValues(t *testing.T) {
	valid := []string{
		"",
		`"+.example.com": https://dns.alidns.com/dns-query`,
		`"+.example.com": [https://doh.pub/dns-query, 223.5.5.5]`,
	}
	for _, value := range valid {
		if err := ValidateResolverPolicyYAML(value); err != nil {
			t.Fatalf("ValidateResolverPolicyYAML(%q) error = %v", value, err)
		}
	}
	invalid := []string{
		"- value",
		"key: {nested: value}",
		"key: true",
		"key: &resolver 1.1.1.1\nother: *resolver",
	}
	for _, value := range invalid {
		if err := ValidateResolverPolicyYAML(value); err == nil {
			t.Fatalf("ValidateResolverPolicyYAML(%q) succeeded", value)
		}
	}
}

func TestValidateIgnoresDormantHostsUntilJeemiManagesThem(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.HostsYAML = "[not-a-mapping]"
	if err := Validate(preferences); err != nil {
		t.Fatalf("disabled hosts should remain dormant: %v", err)
	}
	preferences.DNSUseHosts = true
	if err := Validate(preferences); err == nil {
		t.Fatal("enabled hosts accepted an invalid mapping")
	}
}

func TestValidateIgnoresDormantDNSResourcesUntilEnabled(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.DNSNameserverEnabled = false
	preferences.DNSNameservers = []string{}
	preferences.DNSNameserverPolicyEnabled = false
	preferences.DNSNameserverPolicyYAML = "[not-a-mapping]"
	if err := Validate(preferences); err != nil {
		t.Fatalf("disabled DNS resources should remain dormant: %v", err)
	}
	preferences.DNSNameserverPolicyEnabled = true
	if err := Validate(preferences); err == nil {
		t.Fatal("enabled DNS policy accepted an invalid mapping")
	}
}

func TestValidateIgnoresDormantTUNRouteExclusionsUntilEnabled(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.TUNRouteExcludeAddress = []string{"not-a-cidr"}
	if err := Validate(preferences); err != nil {
		t.Fatalf("disabled TUN route exclusions should remain dormant: %v", err)
	}
	preferences.TUNRouteExcludeAddressEnabled = true
	if err := Validate(preferences); err == nil {
		t.Fatal("enabled TUN route exclusions accepted an invalid CIDR")
	}
}

func TestRuntimeYAMLFragmentValidation(t *testing.T) {
	list := ValidateYAMLFragment(YAMLFragmentInput{
		Field: YAMLFieldDNSNameserver,
		Contents: `- 223.5.5.5
- https://dns.alidns.com/dns-query
- 223.5.5.5`,
	})
	if !list.Valid || !reflect.DeepEqual(list.Values, []string{"223.5.5.5", "https://dns.alidns.com/dns-query"}) || list.NormalizedYAML == "" {
		t.Fatalf("unexpected list validation: %+v", list)
	}

	mapping := ValidateYAMLFragment(YAMLFragmentInput{
		Field:    YAMLFieldHosts,
		Contents: `"dev.local": [127.0.0.1, "::1"]`,
	})
	if !mapping.Valid || mapping.NormalizedYAML == "" {
		t.Fatalf("unexpected mapping validation: %+v", mapping)
	}

	for _, test := range []struct {
		name     string
		field    string
		contents string
	}{
		{name: "blank sequence", field: YAMLFieldDNSProxyServerNameserver, contents: ""},
		{name: "explicit empty sequence", field: YAMLFieldDNSProxyServerNameserver, contents: "[]"},
		{name: "blank mapping", field: YAMLFieldDNSNameserverPolicy, contents: ""},
		{name: "explicit empty mapping", field: YAMLFieldDNSNameserverPolicy, contents: "{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := ValidateYAMLFragment(YAMLFragmentInput{Field: test.field, Contents: test.contents})
			if !result.Valid || result.NormalizedYAML != "" || len(result.Values) != 0 {
				t.Fatalf("empty editor content was not kept blank: %+v", result)
			}
		})
	}

	invalid := ValidateYAMLFragment(YAMLFragmentInput{
		Field:    YAMLFieldDNSFakeIPFilter,
		Contents: "not-a-sequence",
	})
	if invalid.Valid || invalid.Issue == nil || invalid.Issue.Message == "" {
		t.Fatalf("invalid fragment did not return a structured issue: %+v", invalid)
	}

	invalidMapping := ValidateYAMLFragment(YAMLFragmentInput{
		Field:    YAMLFieldHosts,
		Contents: "valid: 127.0.0.1\ninvalid: true",
	})
	if invalidMapping.Valid || invalidMapping.Issue == nil || invalidMapping.Issue.Line != 2 || invalidMapping.Issue.Column < 1 {
		t.Fatalf("mapping error location was not returned: %+v", invalidMapping)
	}

	routeExclusions := ValidateYAMLFragment(YAMLFragmentInput{
		Field: YAMLFieldTUNRouteExcludeAddress,
		Contents: `- 192.168.0.0/16
- fc00::/7
- 203.0.113.8/32`,
	})
	if !routeExclusions.Valid || !reflect.DeepEqual(routeExclusions.Values, []string{"192.168.0.0/16", "fc00::/7", "203.0.113.8/32"}) {
		t.Fatalf("unexpected TUN route exclusion validation: %+v", routeExclusions)
	}

	invalidRouteExclusion := ValidateYAMLFragment(YAMLFragmentInput{
		Field:    YAMLFieldTUNRouteExcludeAddress,
		Contents: "- 192.168.1.1",
	})
	if invalidRouteExclusion.Valid || invalidRouteExclusion.Issue == nil || invalidRouteExclusion.Issue.Line != 1 {
		t.Fatalf("invalid TUN route exclusion was accepted: %+v", invalidRouteExclusion)
	}
}

func TestNormalizeKeepsSemanticallyEmptyMappingsBlank(t *testing.T) {
	preferences := DefaultPreferences()
	preferences.DNSNameserverPolicyYAML = "{}"
	preferences.DNSProxyServerNameserverPolicyYAML = " { } "
	preferences.HostsYAML = "{}\n"

	normalized := Normalize(preferences)
	if normalized.DNSNameserverPolicyYAML != "" ||
		normalized.DNSProxyServerNameserverPolicyYAML != "" ||
		normalized.HostsYAML != "" {
		t.Fatalf("empty mapping syntax remained visible: %+v", normalized)
	}
}

func TestLANBypassRulesAllowEmptyAndRestrictInlineRules(t *testing.T) {
	valid := [][]string{
		{},
		{"DOMAIN,dev.internal,DIRECT"},
		{"IP-CIDR,10.10.0.0/16,DIRECT,no-resolve", "IP-CIDR6,fd00:1234::/48,DIRECT,no-resolve"},
	}
	for _, rules := range valid {
		if err := ValidateLANBypassRules(rules); err != nil {
			t.Fatalf("ValidateLANBypassRules(%#v) error = %v", rules, err)
		}
	}
	invalid := [][]string{
		{"MATCH,DIRECT"},
		{"DOMAIN,dev.internal,Proxy"},
		{"IP-CIDR,10.0.0.0/8,DIRECT"},
		{"IP-CIDR,fd00::/8,DIRECT,no-resolve"},
	}
	for _, rules := range invalid {
		if err := ValidateLANBypassRules(rules); err == nil {
			t.Fatalf("ValidateLANBypassRules(%#v) succeeded", rules)
		}
	}

	empty := ValidateYAMLFragment(YAMLFragmentInput{Field: YAMLFieldLANBypassRules, Contents: "[]"})
	if !empty.Valid || len(empty.Values) != 0 {
		t.Fatalf("empty LAN rule list was rejected: %+v", empty)
	}
}
