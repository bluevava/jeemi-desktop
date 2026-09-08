package runtimeoverride

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"jeemi/internal/config/document"
	"jeemi/internal/runtimeconfig"

	"gopkg.in/yaml.v3"
)

var managedPaths = []string{
	"/mode",
	"/log-level",
	"/ipv6",
	"/allow-lan",
	"/bind-address",
	"/port",
	"/socks-port",
	"/mixed-port",
	"/dns/enable",
	"/dns/listen",
	"/dns/ipv6",
	"/dns/use-hosts",
	"/dns/enhanced-mode",
	"/dns/fake-ip-range",
	"/dns/fake-ip-range6",
	"/dns/fake-ip-filter-mode",
	"/dns/nameserver",
	"/dns/fake-ip-filter",
	"/dns/proxy-server-nameserver",
	"/dns/nameserver-policy",
	"/dns/proxy-server-nameserver-policy",
	"/hosts",
	"/tun/enable",
	"/tun/stack",
	"/tun/device",
	"/tun/auto-route",
	"/tun/auto-detect-interface",
	"/tun/route-exclude-address",
}

// ManagedPaths returns the YAML fields reserved from local configuration
// overlays because Home owns their editing policy. Optional resources retain
// the subscription value when their Home injection switch is off.
func ManagedPaths() []string {
	return append([]string{}, managedPaths...)
}

// Apply injects enabled Home runtime fields after subscription and local
// composition. It preserves unknown and disabled optional fields, and removes
// inactive top-level proxy ports so the selected listener type is unambiguous.
func Apply(configurationYAML []byte, preferences runtimeconfig.Preferences) ([]byte, error) {
	return ApplyForPlatform(configurationYAML, preferences, runtime.GOOS)
}

// ApplyForPlatform is Apply with an explicit target platform. It exists so
// cross-platform TUN naming rules can be verified without running tests on
// every operating system.
func ApplyForPlatform(configurationYAML []byte, preferences runtimeconfig.Preferences, goos string) ([]byte, error) {
	if err := runtimeconfig.Validate(preferences); err != nil {
		return nil, fmt.Errorf("validate runtime preferences: %w", err)
	}
	configuration, err := document.Parse(configurationYAML)
	if err != nil {
		return nil, fmt.Errorf("parse composed configuration: %w", err)
	}
	root := document.Root(configuration)
	for _, path := range []string{"/port", "/socks-port", "/mixed-port"} {
		if err := document.DeleteMappingPath(root, path); err != nil {
			return nil, err
		}
	}
	// The subscription/local value never controls the runtime interface name.
	// On macOS the field is removed because only kernel-assigned utunN names
	// are valid; Windows and Linux receive Jeemi's stable product name.
	if err := document.DeleteMappingPath(root, "/tun/device"); err != nil {
		return nil, err
	}
	listenerPath := map[string]string{
		runtimeconfig.ListenerTypeHTTP:  "/port",
		runtimeconfig.ListenerTypeSOCKS: "/socks-port",
		runtimeconfig.ListenerTypeMixed: "/mixed-port",
	}[preferences.ListenerType]

	values := []struct {
		path  string
		value *yaml.Node
	}{
		{path: "/mode", value: stringNode(preferences.OutboundMode)},
		{path: "/log-level", value: stringNode(preferences.LogLevel)},
		{path: "/ipv6", value: boolNode(preferences.IPv6)},
		{path: "/allow-lan", value: boolNode(preferences.AllowLAN)},
		{path: "/bind-address", value: stringNode(bindAddress(preferences.AllowLAN))},
		{path: listenerPath, value: intNode(preferences.ListenPort)},
		{path: "/dns/enable", value: boolNode(preferences.DNSEnabled)},
		{path: "/dns/listen", value: stringNode(preferences.DNSListen)},
		{path: "/dns/ipv6", value: boolNode(preferences.DNSIPv6)},
		{path: "/dns/enhanced-mode", value: stringNode(preferences.DNSEnhancedMode)},
		{path: "/dns/fake-ip-range", value: stringNode(preferences.DNSFakeIPRange)},
		{path: "/dns/fake-ip-range6", value: stringNode(preferences.DNSFakeIPRange6)},
		{path: "/tun/enable", value: boolNode(preferences.ProxyMode == runtimeconfig.ProxyModeTUN)},
		{path: "/tun/stack", value: stringNode(preferences.TUNStack)},
		{path: "/tun/auto-route", value: boolNode(preferences.TUNAutoRoute)},
		{path: "/tun/auto-detect-interface", value: boolNode(preferences.TUNAutoDetectInterface)},
	}
	for _, item := range values {
		if err := document.SetMappingPath(root, item.path, item.value); err != nil {
			return nil, fmt.Errorf("inject runtime field %s: %w", item.path, err)
		}
	}
	if deviceName := runtimeconfig.TUNDeviceNameForOS(goos); deviceName != "" {
		if err := document.SetMappingPath(root, "/tun/device", stringNode(deviceName)); err != nil {
			return nil, fmt.Errorf("inject runtime field /tun/device: %w", err)
		}
	}
	sequenceValues := []struct {
		path    string
		values  []string
		mode    string
		enabled bool
	}{
		{path: "/tun/route-exclude-address", values: preferences.TUNRouteExcludeAddress, mode: preferences.TUNRouteExcludeAddressMerge, enabled: preferences.TUNRouteExcludeAddressEnabled},
		{path: "/dns/nameserver", values: preferences.DNSNameservers, mode: preferences.DNSNameserverMerge, enabled: preferences.DNSNameserverEnabled},
		{path: "/dns/fake-ip-filter", values: preferences.DNSFakeIPFilter, mode: preferences.DNSFakeIPFilterMerge, enabled: preferences.DNSFakeIPFilterEnabled},
		{path: "/dns/proxy-server-nameserver", values: preferences.DNSProxyServerNameservers, mode: preferences.DNSProxyServerNameserverMerge, enabled: preferences.DNSProxyServerNameserverEnabled},
	}
	for _, item := range sequenceValues {
		if !item.enabled {
			continue
		}
		if err := applyStringSequence(root, item.path, item.values, item.mode); err != nil {
			return nil, fmt.Errorf("inject runtime field %s: %w", item.path, err)
		}
	}
	mappingValues := []struct {
		path     string
		contents string
		mode     string
		enabled  bool
	}{
		{path: "/dns/nameserver-policy", contents: preferences.DNSNameserverPolicyYAML, mode: preferences.DNSNameserverPolicyMerge, enabled: preferences.DNSNameserverPolicyEnabled},
		{path: "/dns/proxy-server-nameserver-policy", contents: preferences.DNSProxyServerNameserverPolicyYAML, mode: preferences.DNSProxyServerNameserverPolicyMerge, enabled: preferences.DNSProxyServerNameserverPolicyEnabled},
	}
	for _, item := range mappingValues {
		if !item.enabled {
			continue
		}
		if err := applyStringMapping(root, item.path, item.contents, item.mode); err != nil {
			return nil, fmt.Errorf("inject runtime field %s: %w", item.path, err)
		}
	}
	if preferences.DNSUseHosts {
		if err := document.SetMappingPath(root, "/dns/use-hosts", boolNode(true)); err != nil {
			return nil, fmt.Errorf("inject runtime field /dns/use-hosts: %w", err)
		}
		if err := applyStringMapping(root, "/hosts", preferences.HostsYAML, preferences.HostsMerge); err != nil {
			return nil, fmt.Errorf("inject runtime field /hosts: %w", err)
		}
	}
	if preferences.DNSFakeIPFilterEnabled {
		if err := document.SetMappingPath(root, "/dns/fake-ip-filter-mode", stringNode(runtimeconfig.DNSFakeIPFilterModeBlacklist)); err != nil {
			return nil, fmt.Errorf("inject runtime field /dns/fake-ip-filter-mode: %w", err)
		}
	}
	if err := prependLANBypassRules(root, preferences.LANBypassRules); err != nil {
		return nil, fmt.Errorf("inject LAN direct rule prefix: %w", err)
	}
	encoded, err := document.Encode(configuration)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func bindAddress(allowLAN bool) string {
	if allowLAN {
		return "*"
	}
	return "127.0.0.1"
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func intNode(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

func stringSequenceNode(values []string) *yaml.Node {
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		sequence.Content = append(sequence.Content, stringNode(value))
	}
	return sequence
}

func applyStringSequence(root *yaml.Node, path string, values []string, mode string) error {
	local := stringSequenceNode(values)
	if mode == runtimeconfig.MergeModeOverride {
		return document.SetMappingPath(root, path, local)
	}
	base, found, err := document.Find(root, path)
	if err != nil {
		return err
	}
	if !found || base.Kind != yaml.SequenceNode {
		return document.SetMappingPath(root, path, local)
	}
	merged := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seen := make(map[string]struct{}, len(base.Content)+len(local.Content))
	for _, source := range [][]*yaml.Node{base.Content, local.Content} {
		for _, item := range source {
			identity, encodeErr := document.EncodeValue(item)
			if encodeErr != nil {
				return encodeErr
			}
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
			merged.Content = append(merged.Content, document.Clone(item))
		}
	}
	return document.SetMappingPath(root, path, merged)
}

func applyStringMapping(root *yaml.Node, path, contents, mode string) error {
	local, err := runtimeconfig.ParseStringMapping(contents)
	if err != nil {
		return err
	}
	if mode == runtimeconfig.MergeModeOverride {
		return document.SetMappingPath(root, path, local)
	}
	base, found, err := document.Find(root, path)
	if err != nil {
		return err
	}
	if !found || base.Kind != yaml.MappingNode {
		return document.SetMappingPath(root, path, local)
	}
	merged := document.Clone(base)
	for index := 0; index < len(local.Content); index += 2 {
		key := local.Content[index]
		value := local.Content[index+1]
		valueIndex := mappingValueIndex(merged, key.Value)
		if valueIndex < 0 {
			merged.Content = append(merged.Content, document.Clone(key), document.Clone(value))
			continue
		}
		merged.Content[valueIndex] = document.Clone(value)
	}
	return document.SetMappingPath(root, path, merged)
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return index + 1
		}
	}
	return -1
}

func prependLANBypassRules(root *yaml.Node, lanBypassRules []string) error {
	rules := stringSequenceNode(lanBypassRules)
	if len(rules.Content) > 0 {
		rules.Content[0].HeadComment = "Jeemi LAN direct rules"
	}
	managed := make(map[string]struct{}, len(lanBypassRules))
	for _, rule := range lanBypassRules {
		managed[strings.ToUpper(strings.TrimSpace(rule))] = struct{}{}
	}
	existing, found, err := document.Find(root, "/rules")
	if err != nil {
		return err
	}
	if found && existing.Kind != yaml.SequenceNode {
		return fmt.Errorf("rules must be a sequence")
	}
	if found {
		for _, rule := range existing.Content {
			if rule.Kind == yaml.ScalarNode {
				if _, duplicate := managed[strings.ToUpper(strings.TrimSpace(rule.Value))]; duplicate {
					continue
				}
			}
			rules.Content = append(rules.Content, document.Clone(rule))
		}
	}
	return document.SetMappingPath(root, "/rules", rules)
}
