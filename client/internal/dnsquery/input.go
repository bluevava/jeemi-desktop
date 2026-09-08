package dnsquery

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

// Domain extracts a hostname from an HTTP(S) URL or a domain with a path.
// Only the hostname reaches DNS; URL paths, queries and fragments are discarded.
func Domain(value string) (string, error) {
	if len(value) > 8192 {
		return "", errors.New("invalid_domain")
	}
	value = strings.TrimSpace(value)
	if schemeEnd := strings.Index(value, "://"); schemeEnd >= 0 && !strings.ContainsAny(value[:schemeEnd], "/?#") {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
			return "", errors.New("invalid_domain")
		}
		value = u.Hostname()
	} else if end := strings.IndexAny(value, "/?#"); end >= 0 {
		value = value[:end]
	}
	return hostname(value)
}

// hostname validates a name without accepting URL syntax. Server uses this
// stricter helper so domain-input convenience cannot weaken DoH validation.
func hostname(value string) (string, error) {
	value, err := idna.Lookup.ToASCII(value)
	value = strings.TrimSuffix(value, ".")
	if err != nil || len(value) == 0 || len(value) > 253 {
		return "", errors.New("invalid_domain")
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return "", errors.New("invalid_domain")
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid_domain")
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
				return "", errors.New("invalid_domain")
			}
		}
	}
	return strings.ToLower(value), nil
}

func Server(value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" && !addr.IsUnspecified() && !addr.IsMulticast() {
		return addr.String(), false, nil
	}
	if len(value) > 2048 || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return "", false, errors.New("invalid_server")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", false, errors.New("invalid_server")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", false, errors.New("invalid_server")
		}
	}
	if _, err := netip.ParseAddr(u.Hostname()); err != nil {
		if _, err := hostname(u.Hostname()); err != nil {
			return "", false, errors.New("invalid_server")
		}
	}
	if u.Path == "" {
		u.Path = "/dns-query"
	}
	return u.String(), true, nil
}
