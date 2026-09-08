package runtimeoverride

import (
	"testing"

	"jeemi/internal/config/document"
	"jeemi/internal/runtimeconfig"

	"gopkg.in/yaml.v3"
)

func TestApplyForceOverridesRuntimeFieldsAndPreservesUnknownValues(t *testing.T) {
	source := []byte(`mode: direct
allow-lan: false
bind-address: 10.0.0.8
port: 8000
socks-port: 8001
mixed-port: 8002
tun:
  enable: false
  stack: system
  auto-route: true
  route-exclude-address:
    - 10.0.0.0/8
future-option: kept
`)
	preferences := runtimeconfig.DefaultPreferences()
	preferences.OutboundMode = runtimeconfig.OutboundModeGlobal
	preferences.ProxyMode = runtimeconfig.ProxyModeTUN
	preferences.ListenerType = runtimeconfig.ListenerTypeSOCKS
	preferences.ListenPort = 1080
	preferences.AllowLAN = true
	preferences.TUNStack = runtimeconfig.TUNStackGVisor
	preferences.TUNRouteExcludeAddress = []string{"192.168.0.0/16", "fc00::/7"}
	preferences.TUNRouteExcludeAddressEnabled = true
	preferences.TUNRouteExcludeAddressMerge = runtimeconfig.MergeModeOverride
	result, err := ApplyForPlatform(source, preferences, "windows")
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, configuration, "/mode", "global")
	assertValue(t, configuration, "/log-level", "silent")
	assertValue(t, configuration, "/ipv6", "true")
	assertValue(t, configuration, "/allow-lan", "true")
	assertValue(t, configuration, "/bind-address", "*")
	assertValue(t, configuration, "/socks-port", "1080")
	assertValue(t, configuration, "/tun/enable", "true")
	assertValue(t, configuration, "/tun/stack", "gvisor")
	assertValue(t, configuration, "/tun/device", runtimeconfig.TUNDeviceName)
	assertValue(t, configuration, "/tun/auto-route", "true")
	assertValue(t, configuration, "/tun/auto-detect-interface", "true")
	assertSequence(t, configuration, "/tun/route-exclude-address", []string{"192.168.0.0/16", "fc00::/7"})
	assertValue(t, configuration, "/dns/enable", "true")
	assertValue(t, configuration, "/dns/listen", runtimeconfig.DefaultDNSListen)
	assertValue(t, configuration, "/dns/ipv6", "false")
	assertValue(t, configuration, "/dns/enhanced-mode", "fake-ip")
	assertValue(t, configuration, "/dns/fake-ip-range", runtimeconfig.DefaultFakeIPRange)
	assertValue(t, configuration, "/dns/fake-ip-range6", runtimeconfig.DefaultFakeIPRange6)
	assertValue(t, configuration, "/future-option", "kept")
	for _, path := range []string{"/port", "/mixed-port"} {
		if _, found, findErr := document.Find(document.Root(configuration), path); findErr != nil || found {
			t.Fatalf("inactive listener %s remains: found=%v err=%v\n%s", path, found, findErr, result)
		}
	}
}

func TestApplyMergesDNSCollectionsAndOnlyManagesHostsWhenEnabled(t *testing.T) {
	source := []byte(`dns:
  use-hosts: false
  nameserver:
    - https://doh.pub/dns-query
    - 223.5.5.5
  fake-ip-filter:
    - +.from-subscription.example
  proxy-server-nameserver:
    - 1.1.1.1
  nameserver-policy:
    "+.shared.example": 1.1.1.1
    "+.subscription.example": 8.8.8.8
  proxy-server-nameserver-policy:
    "+.node.shared.example": 1.0.0.1
    "+.node.subscription.example": 8.8.4.4
hosts:
  subscription.local: 192.168.1.10
rules:
  - MATCH,DIRECT
`)
	preferences := runtimeconfig.DefaultPreferences()
	preferences.DNSNameservers = []string{"223.5.5.5", "114.114.114.114"}
	preferences.DNSFakeIPFilter = []string{"+.lan", "+.local"}
	preferences.DNSProxyServerNameservers = []string{"https://dns.alidns.com/dns-query"}
	preferences.DNSProxyServerNameserverEnabled = true
	preferences.DNSNameserverPolicyYAML = `"+.shared.example": 223.5.5.5
"+.runtime.example": https://dns.alidns.com/dns-query`
	preferences.DNSNameserverPolicyEnabled = true
	preferences.DNSProxyServerNameserverPolicyYAML = `"+.node.shared.example": 223.6.6.6
"+.node.runtime.example": https://doh.pub/dns-query`
	preferences.DNSProxyServerNameserverPolicyEnabled = true
	preferences.HostsYAML = `"runtime.local": 192.168.1.20`

	withoutHosts, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(withoutHosts)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, configuration, "/dns/use-hosts", "false")
	assertValue(t, configuration, "/hosts/subscription.local", "192.168.1.10")
	assertMissing(t, configuration, "/hosts/runtime.local")
	assertSequence(t, configuration, "/dns/nameserver", []string{"https://doh.pub/dns-query", "223.5.5.5", "114.114.114.114"})
	assertSequence(t, configuration, "/dns/fake-ip-filter", []string{"+.lan", "+.local"})
	assertSequence(t, configuration, "/dns/proxy-server-nameserver", []string{"1.1.1.1", "https://dns.alidns.com/dns-query"})
	assertValue(t, configuration, "/dns/nameserver-policy/+.shared.example", "223.5.5.5")
	assertValue(t, configuration, "/dns/nameserver-policy/+.subscription.example", "8.8.8.8")
	assertValue(t, configuration, "/dns/nameserver-policy/+.runtime.example", "https://dns.alidns.com/dns-query")
	assertValue(t, configuration, "/dns/proxy-server-nameserver-policy/+.node.shared.example", "223.6.6.6")
	assertValue(t, configuration, "/dns/proxy-server-nameserver-policy/+.node.subscription.example", "8.8.4.4")
	assertValue(t, configuration, "/dns/proxy-server-nameserver-policy/+.node.runtime.example", "https://doh.pub/dns-query")

	preferences.DNSUseHosts = true
	withHosts, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err = document.Parse(withHosts)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, configuration, "/dns/use-hosts", "true")
	assertValue(t, configuration, "/hosts/subscription.local", "192.168.1.10")
	assertValue(t, configuration, "/hosts/runtime.local", "192.168.1.20")
}

func TestApplyLeavesDisabledDNSResourcesUntouched(t *testing.T) {
	source := []byte(`dns:
  fake-ip-filter-mode: whitelist
  nameserver:
    - 9.9.9.9
  fake-ip-filter:
    - +.subscription.example
  proxy-server-nameserver:
    - 1.1.1.1
  nameserver-policy:
    "+.subscription.example": 8.8.8.8
  proxy-server-nameserver-policy:
    "+.node.example": 8.8.4.4
rules: []
`)
	preferences := runtimeconfig.DefaultPreferences()
	preferences.DNSNameserverEnabled = false
	preferences.DNSFakeIPFilterEnabled = false
	preferences.DNSProxyServerNameserverEnabled = false
	preferences.DNSNameserverPolicyEnabled = false
	preferences.DNSProxyServerNameserverPolicyEnabled = false

	result, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, configuration, "/dns/nameserver", []string{"9.9.9.9"})
	assertSequence(t, configuration, "/dns/fake-ip-filter", []string{"+.subscription.example"})
	assertSequence(t, configuration, "/dns/proxy-server-nameserver", []string{"1.1.1.1"})
	assertValue(t, configuration, "/dns/fake-ip-filter-mode", "whitelist")
	assertValue(t, configuration, "/dns/nameserver-policy/+.subscription.example", "8.8.8.8")
	assertValue(t, configuration, "/dns/proxy-server-nameserver-policy/+.node.example", "8.8.4.4")
}

func TestApplyPrependsManagedLANRulesAndRemovesDuplicates(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	managedLANBypassRules := preferences.LANBypassRules
	result, err := Apply([]byte(`rules:
  - DOMAIN-SUFFIX,example.com,Proxy
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - MATCH,Proxy
`), preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	rules, found, err := document.Find(document.Root(configuration), "/rules")
	if err != nil || !found || rules.Kind != yaml.SequenceNode {
		t.Fatalf("rules missing: found=%v err=%v", found, err)
	}
	if len(rules.Content) != len(managedLANBypassRules)+2 {
		t.Fatalf("rules count = %d, want %d\n%s", len(rules.Content), len(managedLANBypassRules)+2, result)
	}
	for index, expected := range managedLANBypassRules {
		if rules.Content[index].Value != expected {
			t.Fatalf("rule[%d] = %q, want %q", index, rules.Content[index].Value, expected)
		}
	}
	if got := rules.Content[len(managedLANBypassRules)].Value; got != "DOMAIN-SUFFIX,example.com,Proxy" {
		t.Fatalf("first subscription rule = %q", got)
	}
	if got := rules.Content[len(rules.Content)-1].Value; got != "MATCH,Proxy" {
		t.Fatalf("terminal rule = %q", got)
	}
}

func TestApplyUsesEditableLANRulePrefixAndAllowsEmptyList(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	preferences.LANBypassRules = []string{"DOMAIN,dev.internal,DIRECT"}
	result, err := Apply([]byte("rules:\n  - MATCH,Proxy\n"), preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, configuration, "/rules", []string{"DOMAIN,dev.internal,DIRECT", "MATCH,Proxy"})

	preferences.LANBypassRules = []string{}
	result, err = Apply([]byte("rules:\n  - MATCH,Proxy\n"), preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err = document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, configuration, "/rules", []string{"MATCH,Proxy"})
}

func TestApplyRejectsNonSequenceRulesInsteadOfDiscardingThem(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	if result, err := Apply([]byte("rules: MATCH,DIRECT\n"), preferences); err == nil || result != nil {
		t.Fatalf("Apply() result=%q err=%v", result, err)
	}
}

func TestApplyHonorsConfigurableTUNRoutingFlags(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	preferences.TUNAutoRoute = false
	preferences.TUNAutoDetectInterface = false
	preferences.TUNRouteExcludeAddress = []string{}
	preferences.TUNRouteExcludeAddressEnabled = true
	preferences.TUNRouteExcludeAddressMerge = runtimeconfig.MergeModeOverride
	result, err := Apply([]byte("tun:\n  route-exclude-address: [10.0.0.0/8]\nrules: []\n"), preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, configuration, "/tun/auto-route", "false")
	assertValue(t, configuration, "/tun/auto-detect-interface", "false")
	assertSequence(t, configuration, "/tun/route-exclude-address", []string{})
}

func TestApplyCombinesTUNRouteExclusionsOnlyWhenEnabled(t *testing.T) {
	source := []byte(`tun:
  route-exclude-address:
    - 10.0.0.0/8
    - 192.168.0.0/16
rules: []
`)
	preferences := runtimeconfig.DefaultPreferences()
	preferences.TUNRouteExcludeAddress = []string{"10.0.0.0/8", "fc00::/7"}

	disabledResult, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	disabledConfiguration, err := document.Parse(disabledResult)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, disabledConfiguration, "/tun/route-exclude-address", []string{"10.0.0.0/8", "192.168.0.0/16"})

	preferences.TUNRouteExcludeAddressEnabled = true
	preferences.TUNRouteExcludeAddressMerge = runtimeconfig.MergeModeAppend
	appendedResult, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	appendedConfiguration, err := document.Parse(appendedResult)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, appendedConfiguration, "/tun/route-exclude-address", []string{"10.0.0.0/8", "192.168.0.0/16", "fc00::/7"})

	preferences.TUNRouteExcludeAddressMerge = runtimeconfig.MergeModeOverride
	overriddenResult, err := Apply(source, preferences)
	if err != nil {
		t.Fatal(err)
	}
	overriddenConfiguration, err := document.Parse(overriddenResult)
	if err != nil {
		t.Fatal(err)
	}
	assertSequence(t, overriddenConfiguration, "/tun/route-exclude-address", []string{"10.0.0.0/8", "fc00::/7"})
}

func TestApplyUsesPlatformSafeTUNDeviceNames(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ProxyMode = runtimeconfig.ProxyModeTUN

	windowsResult, err := ApplyForPlatform([]byte("tun:\n  device: from-subscription\n"), preferences, "windows")
	if err != nil {
		t.Fatal(err)
	}
	windowsConfiguration, err := document.Parse(windowsResult)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, windowsConfiguration, "/tun/device", "JeemiTun")

	darwinResult, err := ApplyForPlatform([]byte("tun:\n  device: from-subscription\n"), preferences, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	darwinConfiguration, err := document.Parse(darwinResult)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, findErr := document.Find(document.Root(darwinConfiguration), "/tun/device"); findErr != nil || found {
		t.Fatalf("macOS tun.device must be omitted: found=%v err=%v\n%s", found, findErr, darwinResult)
	}
}

func TestApplyUsesLoopbackAndDisablesTUNForSystemProxy(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ListenerType = runtimeconfig.ListenerTypeHTTP
	result, err := Apply([]byte("proxies: []\n"), preferences)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := document.Parse(result)
	if err != nil {
		t.Fatal(err)
	}
	assertValue(t, configuration, "/port", "7890")
	assertValue(t, configuration, "/bind-address", "127.0.0.1")
	assertValue(t, configuration, "/allow-lan", "false")
	assertValue(t, configuration, "/tun/enable", "false")
}

func TestApplyKeepsOnlyTheSelectedTopLevelListener(t *testing.T) {
	tests := []struct {
		listenerType string
		selectedPath string
	}{
		{listenerType: runtimeconfig.ListenerTypeHTTP, selectedPath: "/port"},
		{listenerType: runtimeconfig.ListenerTypeSOCKS, selectedPath: "/socks-port"},
		{listenerType: runtimeconfig.ListenerTypeMixed, selectedPath: "/mixed-port"},
	}
	for _, test := range tests {
		t.Run(test.listenerType, func(t *testing.T) {
			preferences := runtimeconfig.DefaultPreferences()
			preferences.ListenerType = test.listenerType
			preferences.ListenPort = 7898
			result, err := Apply([]byte("port: 1\nsocks-port: 2\nmixed-port: 3\n"), preferences)
			if err != nil {
				t.Fatal(err)
			}
			configuration, err := document.Parse(result)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/port", "/socks-port", "/mixed-port"} {
				value, found, findErr := document.Find(document.Root(configuration), path)
				if findErr != nil {
					t.Fatal(findErr)
				}
				if path == test.selectedPath {
					if !found || value.Value != "7898" {
						t.Fatalf("selected listener %s = %+v, found=%v", path, value, found)
					}
				} else if found {
					t.Fatalf("inactive listener %s remains in:\n%s", path, result)
				}
			}
		})
	}
}

func TestApplyRejectsInvalidPreferencesWithoutReturningConfiguration(t *testing.T) {
	preferences := runtimeconfig.DefaultPreferences()
	preferences.ListenPort = 0
	if result, err := Apply([]byte("mode: rule\n"), preferences); err == nil || result != nil {
		t.Fatalf("Apply() result=%q err=%v", result, err)
	}
}

func assertValue(t *testing.T, configuration *yaml.Node, path, expected string) {
	t.Helper()
	value, found, err := document.Find(document.Root(configuration), path)
	if err != nil || !found || value.Value != expected {
		t.Fatalf("value at %s = %+v, found=%v err=%v, want %q", path, value, found, err, expected)
	}
}

func assertMissing(t *testing.T, configuration *yaml.Node, path string) {
	t.Helper()
	if _, found, err := document.Find(document.Root(configuration), path); err != nil || found {
		t.Fatalf("value at %s found=%v err=%v", path, found, err)
	}
}

func assertSequence(t *testing.T, configuration *yaml.Node, path string, expected []string) {
	t.Helper()
	value, found, err := document.Find(document.Root(configuration), path)
	if err != nil || !found || value.Kind != yaml.SequenceNode {
		t.Fatalf("sequence at %s = %+v, found=%v err=%v", path, value, found, err)
	}
	if len(value.Content) != len(expected) {
		t.Fatalf("sequence at %s has %d entries, want %d", path, len(value.Content), len(expected))
	}
	for index, item := range value.Content {
		if item.Value != expected[index] {
			t.Fatalf("sequence at %s[%d] = %q, want %q", path, index, item.Value, expected[index])
		}
	}
}
