package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
)

const (
	OutboundModeRule   = "rule"
	OutboundModeGlobal = "global"
	OutboundModeDirect = "direct"

	ProxyModeSystemProxy = "system_proxy"
	ProxyModeTUN         = "tun"

	ListenerTypeHTTP  = "http"
	ListenerTypeSOCKS = "socks"
	ListenerTypeMixed = "mixed"

	TUNStackSystem = "system"
	TUNStackGVisor = "gvisor"
	TUNStackMixed  = "mixed"

	LogLevelSilent  = "silent"
	LogLevelError   = "error"
	LogLevelWarning = "warning"
	LogLevelInfo    = "info"
	LogLevelDebug   = "debug"

	FindProcessModeAlways = "always"
	FindProcessModeStrict = "strict"
	FindProcessModeOff    = "off"

	DNSEnhancedModeFakeIP        = "fake-ip"
	DNSEnhancedModeRedirHost     = "redir-host"
	DNSFakeIPFilterModeBlacklist = "blacklist"

	MergeModeAppend   = "append"
	MergeModeOverride = "override"

	// TUNDeviceName is Jeemi's stable TUN interface name on platforms that
	// permit applications to choose it. macOS is the one exception: the
	// kernel/mihomo requires an automatically assigned utunN name.
	TUNDeviceName = "JeemiTun"

	DefaultListenPort       = 7890
	DefaultDNSListen        = "127.0.0.1:1053"
	DefaultFakeIPRange      = "198.18.0.1/16"
	DefaultFakeIPRange6     = "fdfe:dcba:9876::1/64"
	maxResolverCount        = 128
	maxFakeIPFilterCount    = 1024
	maxTUNRouteExcludeCount = 1024
	maxResolverLength       = 2048
	maxMappingYAMLBytes     = 64 << 10
	maxRuntimeStringBytes   = 4096
)

var (
	defaultDNSNameservers = []string{
		"114.114.114.114",
		"223.5.5.5",
	}
	defaultDNSFakeIPFilter = []string{
		"+.lan",
		"+.local",
		"+.home.arpa",
		"+.localdomain",
		"+.localhost",
	}
	defaultLANBypassRules = []string{
		"DOMAIN,localhost,DIRECT",
		"DOMAIN-SUFFIX,lan,DIRECT",
		"DOMAIN-SUFFIX,local,DIRECT",
		"DOMAIN-SUFFIX,localdomain,DIRECT",
		"DOMAIN-SUFFIX,home.arpa,DIRECT",
		"IP-CIDR,0.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,224.0.0.0/3,DIRECT,no-resolve",
		"IP-CIDR6,::/128,DIRECT,no-resolve",
		"IP-CIDR6,::1/128,DIRECT,no-resolve",
		"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
		"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
		"IP-CIDR6,ff00::/8,DIRECT,no-resolve",
	}
)

// DefaultLANBypassRules returns a detached copy of Jeemi's current built-in
// direct-rule prefix. The UI uses this through a narrow read-only binding when
// the user asks to restore the editor to system defaults.
func DefaultLANBypassRules() []string {
	return append([]string{}, defaultLANBypassRules...)
}

// TUNDeviceNameForOS returns the concrete name that may be written to
// tun.device. An empty result means the field must be omitted so mihomo can
// request a valid platform-assigned interface name.
func TUNDeviceNameForOS(goos string) string {
	switch goos {
	case "windows", "linux":
		return TUNDeviceName
	default:
		return ""
	}
}

// Preferences are persisted desired client choices that Jeemi injects after
// subscription and local-configuration composition. The application
// coordinator may apply a valid change to a running core, but only its
// desired/active status can claim whether that application succeeded.
//
// DNSUseHosts deliberately means "Jeemi manages use-hosts and hosts". When it
// is false both source fields remain untouched, matching the Home switch's
// opt-in semantics instead of forcibly disabling subscription hosts.
type Preferences struct {
	OutboundMode string `json:"outboundMode"`
	ProxyMode    string `json:"proxyMode"`
	ListenerType string `json:"listenerType"`
	ListenPort   int    `json:"listenPort"`
	AllowLAN     bool   `json:"allowLan"`
	TUNStack     string `json:"tunStack"`

	LogLevel                      string   `json:"logLevel"`
	FindProcessMode               string   `json:"findProcessMode"`
	IPv6                          bool     `json:"ipv6"`
	DNSEnabled                    bool     `json:"dnsEnabled"`
	DNSListen                     string   `json:"dnsListen"`
	DNSIPv6                       bool     `json:"dnsIpv6"`
	DNSUseHosts                   bool     `json:"dnsUseHosts"`
	DNSEnhancedMode               string   `json:"dnsEnhancedMode"`
	DNSFakeIPRange                string   `json:"dnsFakeIpRange"`
	DNSFakeIPRange6               string   `json:"dnsFakeIpRange6"`
	TUNAutoRoute                  bool     `json:"tunAutoRoute"`
	TUNAutoDetectInterface        bool     `json:"tunAutoDetectInterface"`
	TUNRouteExcludeAddress        []string `json:"tunRouteExcludeAddress"`
	TUNRouteExcludeAddressEnabled bool     `json:"tunRouteExcludeAddressEnabled"`
	TUNRouteExcludeAddressMerge   string   `json:"tunRouteExcludeAddressMerge"`

	DNSNameservers                        []string `json:"dnsNameservers"`
	DNSNameserverEnabled                  bool     `json:"dnsNameserverEnabled"`
	DNSNameserverMerge                    string   `json:"dnsNameserverMerge"`
	DNSFakeIPFilter                       []string `json:"dnsFakeIpFilter"`
	DNSFakeIPFilterEnabled                bool     `json:"dnsFakeIpFilterEnabled"`
	DNSFakeIPFilterMerge                  string   `json:"dnsFakeIpFilterMerge"`
	DNSProxyServerNameservers             []string `json:"dnsProxyServerNameservers"`
	DNSProxyServerNameserverEnabled       bool     `json:"dnsProxyServerNameserverEnabled"`
	DNSProxyServerNameserverMerge         string   `json:"dnsProxyServerNameserverMerge"`
	DNSNameserverPolicyYAML               string   `json:"dnsNameserverPolicyYaml"`
	DNSNameserverPolicyEnabled            bool     `json:"dnsNameserverPolicyEnabled"`
	DNSNameserverPolicyMerge              string   `json:"dnsNameserverPolicyMerge"`
	DNSProxyServerNameserverPolicyYAML    string   `json:"dnsProxyServerNameserverPolicyYaml"`
	DNSProxyServerNameserverPolicyEnabled bool     `json:"dnsProxyServerNameserverPolicyEnabled"`
	DNSProxyServerNameserverPolicyMerge   string   `json:"dnsProxyServerNameserverPolicyMerge"`
	HostsYAML                             string   `json:"hostsYaml"`
	HostsMerge                            string   `json:"hostsMerge"`
	LANBypassRules                        []string `json:"lanBypassRules"`
}

func DefaultPreferences() Preferences {
	return Preferences{
		OutboundMode: OutboundModeRule,
		ProxyMode:    ProxyModeTUN,
		ListenerType: ListenerTypeMixed,
		ListenPort:   DefaultListenPort,
		AllowLAN:     false,
		TUNStack:     TUNStackMixed,

		LogLevel:                      LogLevelSilent,
		FindProcessMode:               FindProcessModeStrict,
		IPv6:                          true,
		DNSEnabled:                    true,
		DNSListen:                     DefaultDNSListen,
		DNSIPv6:                       false,
		DNSUseHosts:                   false,
		DNSEnhancedMode:               DNSEnhancedModeFakeIP,
		DNSFakeIPRange:                DefaultFakeIPRange,
		DNSFakeIPRange6:               DefaultFakeIPRange6,
		TUNAutoRoute:                  true,
		TUNAutoDetectInterface:        true,
		TUNRouteExcludeAddress:        []string{},
		TUNRouteExcludeAddressEnabled: false,
		TUNRouteExcludeAddressMerge:   MergeModeAppend,

		DNSNameservers:                        append([]string{}, defaultDNSNameservers...),
		DNSNameserverEnabled:                  true,
		DNSNameserverMerge:                    MergeModeAppend,
		DNSFakeIPFilter:                       append([]string{}, defaultDNSFakeIPFilter...),
		DNSFakeIPFilterEnabled:                true,
		DNSFakeIPFilterMerge:                  MergeModeOverride,
		DNSProxyServerNameservers:             []string{},
		DNSProxyServerNameserverEnabled:       false,
		DNSProxyServerNameserverMerge:         MergeModeAppend,
		DNSNameserverPolicyYAML:               "",
		DNSNameserverPolicyEnabled:            false,
		DNSNameserverPolicyMerge:              MergeModeAppend,
		DNSProxyServerNameserverPolicyYAML:    "",
		DNSProxyServerNameserverPolicyEnabled: false,
		DNSProxyServerNameserverPolicyMerge:   MergeModeAppend,
		HostsYAML:                             "",
		HostsMerge:                            MergeModeAppend,
		LANBypassRules:                        DefaultLANBypassRules(),
	}
}

// UnmarshalJSON applies safe defaults to omitted fields while preserving
// explicit false values and empty lists. Stored values never enable a separate
// injection switch implicitly.
func (preferences *Preferences) UnmarshalJSON(contents []byte) error {
	type plainPreferences Preferences
	decoded := plainPreferences(DefaultPreferences())
	if err := json.Unmarshal(contents, &decoded); err != nil {
		return err
	}
	*preferences = Preferences(decoded)
	return nil
}

func Normalize(preferences Preferences) Preferences {
	defaults := DefaultPreferences()
	if reflect.DeepEqual(preferences, Preferences{}) {
		return defaults
	}
	if !validOutboundMode(preferences.OutboundMode) {
		preferences.OutboundMode = defaults.OutboundMode
	}
	if !validProxyMode(preferences.ProxyMode) {
		preferences.ProxyMode = defaults.ProxyMode
	}
	if !validListenerType(preferences.ListenerType) {
		preferences.ListenerType = defaults.ListenerType
	}
	if preferences.ListenPort < 1 || preferences.ListenPort > 65535 {
		preferences.ListenPort = defaults.ListenPort
	}
	if !validTUNStack(preferences.TUNStack) {
		preferences.TUNStack = defaults.TUNStack
	}
	if !validLogLevel(preferences.LogLevel) {
		preferences.LogLevel = defaults.LogLevel
	}
	if !validFindProcessMode(preferences.FindProcessMode) {
		preferences.FindProcessMode = defaults.FindProcessMode
	}
	if err := validateListenAddress(preferences.DNSListen, preferences.ListenPort, preferences.DNSEnabled); err != nil {
		preferences.DNSListen = defaults.DNSListen
	}
	if !validDNSEnhancedMode(preferences.DNSEnhancedMode) {
		preferences.DNSEnhancedMode = defaults.DNSEnhancedMode
	}
	if err := validateIPPrefix(preferences.DNSFakeIPRange, false); err != nil {
		preferences.DNSFakeIPRange = defaults.DNSFakeIPRange
	}
	if err := validateIPPrefix(preferences.DNSFakeIPRange6, true); err != nil {
		preferences.DNSFakeIPRange6 = defaults.DNSFakeIPRange6
	}
	if preferences.DNSNameservers == nil {
		preferences.DNSNameservers = append([]string{}, defaults.DNSNameservers...)
	} else {
		preferences.DNSNameservers = normalizeStringList(preferences.DNSNameservers)
	}
	if preferences.DNSFakeIPFilter == nil {
		preferences.DNSFakeIPFilter = append([]string{}, defaults.DNSFakeIPFilter...)
	} else {
		preferences.DNSFakeIPFilter = normalizeStringList(preferences.DNSFakeIPFilter)
	}
	if preferences.DNSProxyServerNameservers == nil {
		preferences.DNSProxyServerNameservers = []string{}
	} else {
		preferences.DNSProxyServerNameservers = normalizeStringList(preferences.DNSProxyServerNameservers)
	}
	if preferences.LANBypassRules == nil {
		preferences.LANBypassRules = DefaultLANBypassRules()
	} else {
		preferences.LANBypassRules = normalizeStringList(preferences.LANBypassRules)
	}
	if preferences.TUNRouteExcludeAddress == nil {
		preferences.TUNRouteExcludeAddress = []string{}
	} else {
		preferences.TUNRouteExcludeAddress = normalizeStringList(preferences.TUNRouteExcludeAddress)
	}
	for target, fallback := range map[*string]string{
		&preferences.TUNRouteExcludeAddressMerge:         defaults.TUNRouteExcludeAddressMerge,
		&preferences.DNSNameserverMerge:                  defaults.DNSNameserverMerge,
		&preferences.DNSFakeIPFilterMerge:                defaults.DNSFakeIPFilterMerge,
		&preferences.DNSProxyServerNameserverMerge:       defaults.DNSProxyServerNameserverMerge,
		&preferences.DNSNameserverPolicyMerge:            defaults.DNSNameserverPolicyMerge,
		&preferences.DNSProxyServerNameserverPolicyMerge: defaults.DNSProxyServerNameserverPolicyMerge,
		&preferences.HostsMerge:                          defaults.HostsMerge,
	} {
		if !validMergeMode(*target) {
			*target = fallback
		}
	}
	preferences.DNSNameserverPolicyYAML = normalizeMappingEditorYAML(preferences.DNSNameserverPolicyYAML)
	preferences.DNSProxyServerNameserverPolicyYAML = normalizeMappingEditorYAML(preferences.DNSProxyServerNameserverPolicyYAML)
	preferences.HostsYAML = normalizeMappingEditorYAML(preferences.HostsYAML)
	return preferences
}

func Validate(preferences Preferences) error {
	if !validOutboundMode(preferences.OutboundMode) {
		return fmt.Errorf("outbound mode is invalid")
	}
	if !validProxyMode(preferences.ProxyMode) {
		return fmt.Errorf("proxy mode is invalid")
	}
	if !validListenerType(preferences.ListenerType) {
		return fmt.Errorf("listener type is invalid")
	}
	if preferences.ListenPort < 1 || preferences.ListenPort > 65535 {
		return fmt.Errorf("listen port must be between 1 and 65535")
	}
	if !validTUNStack(preferences.TUNStack) {
		return fmt.Errorf("TUN stack is invalid")
	}
	if !validLogLevel(preferences.LogLevel) {
		return fmt.Errorf("log level is invalid")
	}
	if !validFindProcessMode(preferences.FindProcessMode) {
		return fmt.Errorf("find process mode is invalid")
	}
	if err := validateListenAddress(preferences.DNSListen, preferences.ListenPort, preferences.DNSEnabled); err != nil {
		return err
	}
	if !validDNSEnhancedMode(preferences.DNSEnhancedMode) {
		return fmt.Errorf("DNS enhanced mode is invalid")
	}
	if err := validateIPPrefix(preferences.DNSFakeIPRange, false); err != nil {
		return fmt.Errorf("DNS fake IP range is invalid: %w", err)
	}
	if err := validateIPPrefix(preferences.DNSFakeIPRange6, true); err != nil {
		return fmt.Errorf("DNS fake IPv6 range is invalid: %w", err)
	}
	if preferences.DNSNameserverEnabled {
		if err := validateStringList("DNS nameserver", preferences.DNSNameservers, 1, maxResolverCount); err != nil {
			return err
		}
	}
	if preferences.DNSFakeIPFilterEnabled {
		if err := validateStringList("DNS fake IP filter", preferences.DNSFakeIPFilter, 0, maxFakeIPFilterCount); err != nil {
			return err
		}
	}
	if preferences.DNSProxyServerNameserverEnabled {
		if err := validateStringList("DNS proxy server nameserver", preferences.DNSProxyServerNameservers, 0, maxResolverCount); err != nil {
			return err
		}
	}
	for label, mode := range map[string]string{
		"TUN route exclude address merge mode":          preferences.TUNRouteExcludeAddressMerge,
		"DNS nameserver merge mode":                     preferences.DNSNameserverMerge,
		"DNS fake IP filter merge mode":                 preferences.DNSFakeIPFilterMerge,
		"DNS proxy server nameserver merge mode":        preferences.DNSProxyServerNameserverMerge,
		"DNS nameserver policy merge mode":              preferences.DNSNameserverPolicyMerge,
		"DNS proxy server nameserver policy merge mode": preferences.DNSProxyServerNameserverPolicyMerge,
		"hosts merge mode":                              preferences.HostsMerge,
	} {
		if !validMergeMode(mode) {
			return fmt.Errorf("%s is invalid", label)
		}
	}
	if preferences.DNSNameserverPolicyEnabled {
		if err := ValidateResolverPolicyYAML(preferences.DNSNameserverPolicyYAML); err != nil {
			return fmt.Errorf("DNS nameserver policy is invalid: %w", err)
		}
	}
	if preferences.DNSProxyServerNameserverPolicyEnabled {
		if err := ValidateResolverPolicyYAML(preferences.DNSProxyServerNameserverPolicyYAML); err != nil {
			return fmt.Errorf("DNS proxy server nameserver policy is invalid: %w", err)
		}
	}
	if preferences.DNSUseHosts {
		if err := ValidateHostsYAML(preferences.HostsYAML); err != nil {
			return fmt.Errorf("hosts is invalid: %w", err)
		}
	}
	if err := ValidateLANBypassRules(preferences.LANBypassRules); err != nil {
		return fmt.Errorf("LAN bypass rules are invalid: %w", err)
	}
	if preferences.TUNRouteExcludeAddressEnabled {
		if err := ValidateTUNRouteExcludeAddresses(preferences.TUNRouteExcludeAddress); err != nil {
			return fmt.Errorf("TUN route exclude addresses are invalid: %w", err)
		}
	}
	return nil
}

func Equal(left, right Preferences) bool {
	return reflect.DeepEqual(left, right)
}

func validOutboundMode(value string) bool {
	return value == OutboundModeRule || value == OutboundModeGlobal || value == OutboundModeDirect
}

func validProxyMode(value string) bool {
	return value == ProxyModeSystemProxy || value == ProxyModeTUN
}

func validListenerType(value string) bool {
	return value == ListenerTypeHTTP || value == ListenerTypeSOCKS || value == ListenerTypeMixed
}

func validTUNStack(value string) bool {
	return value == TUNStackSystem || value == TUNStackGVisor || value == TUNStackMixed
}

func validLogLevel(value string) bool {
	return value == LogLevelSilent || value == LogLevelError || value == LogLevelWarning || value == LogLevelInfo || value == LogLevelDebug
}

func validFindProcessMode(value string) bool {
	return value == FindProcessModeAlways || value == FindProcessModeStrict || value == FindProcessModeOff
}

func validDNSEnhancedMode(value string) bool {
	return value == DNSEnhancedModeFakeIP || value == DNSEnhancedModeRedirHost
}

func validMergeMode(value string) bool {
	return value == MergeModeAppend || value == MergeModeOverride
}

func validateListenAddress(value string, proxyPort int, enabled bool) error {
	if len(value) > maxRuntimeStringBytes {
		return fmt.Errorf("DNS listen address is too long")
	}
	host, portText, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("DNS listen address must use host:port format")
	}
	if host != "" && host != "*" && net.ParseIP(host) == nil {
		return fmt.Errorf("DNS listen host must be an IP address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("DNS listen port must be between 1 and 65535")
	}
	if enabled && port == proxyPort {
		return fmt.Errorf("DNS listen port must differ from the proxy listen port")
	}
	return nil
}

func validateIPPrefix(value string, requireIPv6 bool) error {
	if len(value) > maxRuntimeStringBytes {
		return fmt.Errorf("CIDR is too long")
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	if requireIPv6 && !prefix.Addr().Is6() {
		return fmt.Errorf("must be an IPv6 prefix")
	}
	if !requireIPv6 && !prefix.Addr().Is4() {
		return fmt.Errorf("must be an IPv4 prefix")
	}
	return nil
}

func validateStringList(label string, values []string, minimum, maximum int) error {
	if values == nil || len(values) < minimum || len(values) > maximum {
		return fmt.Errorf("%s must contain between %d and %d entries", label, minimum, maximum)
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || len(trimmed) > maxResolverLength || strings.ContainsAny(trimmed, "\r\n\x00") {
			return fmt.Errorf("%s contains an invalid entry", label)
		}
	}
	return nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
