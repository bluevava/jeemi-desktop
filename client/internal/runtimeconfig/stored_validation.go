package runtimeconfig

import "jeemi/internal/configfile"

// Unknown routing enums are capabilities, not a request to silently use a
// different outbound policy. UI submissions continue to use Validate directly.
func ValidateStored(p Preferences) error {
	if !validOutboundMode(p.OutboundMode) || !validProxyMode(p.ProxyMode) ||
		!validListenerType(p.ListenerType) || !validTUNStack(p.TUNStack) ||
		!validLogLevel(p.LogLevel) || !validFindProcessMode(p.FindProcessMode) ||
		!validDNSEnhancedMode(p.DNSEnhancedMode) {
		return configfile.UnsupportedFormat("stored runtime option is unsupported")
	}
	for _, mode := range []string{p.TUNRouteExcludeAddressMerge, p.DNSNameserverMerge, p.DNSFakeIPFilterMerge, p.DNSProxyServerNameserverMerge, p.DNSNameserverPolicyMerge, p.DNSProxyServerNameserverPolicyMerge, p.HostsMerge} {
		if !validMergeMode(mode) {
			return configfile.UnsupportedFormat("stored runtime merge mode is unsupported")
		}
	}
	return Validate(p)
}
