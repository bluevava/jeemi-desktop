package subscriptionformat

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

func parseHostURI(raw string, line int) (dnsPolicy, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Path != "" || u.Fragment != "" {
		return dnsPolicy{}, failure("invalid_dns", line, "")
	}
	pattern, err := normalizePattern(u.Host, false)
	if err != nil {
		return dnsPolicy{}, failure("invalid_dns", line, "domain")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) != 1 || len(query["server"]) == 0 {
		return dnsPolicy{}, failure("invalid_dns", line, "server")
	}
	policy := dnsPolicy{Pattern: pattern, Line: line, Nodes: true}
	for _, value := range query["server"] {
		server, err := normalizeResolver(value)
		if err != nil {
			return dnsPolicy{}, failure("invalid_dns", line, "server")
		}
		policy.Servers = appendUnique(policy.Servers, server)
	}
	return policy, nil
}

func normalizePattern(value string, surge bool) (string, error) {
	prefix := ""
	switch {
	case strings.HasPrefix(value, "*."):
		prefix = "*."
		value = value[2:]
		if surge {
			prefix = "."
		}
	case strings.HasPrefix(value, "+."):
		prefix = "+."
		value = value[2:]
	case strings.HasPrefix(value, "."):
		prefix = "."
		value = value[1:]
	}
	domain, err := normalizeServer(value)
	if err != nil || net.ParseIP(domain) != nil {
		return "", failure("invalid_dns", 0, "domain")
	}
	return prefix + domain, nil
}

func matchesDomain(pattern, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasPrefix(pattern, "+.") {
		root := pattern[2:]
		return host == root || strings.HasSuffix(host, "."+root)
	}
	if strings.HasPrefix(pattern, "*.") {
		root := pattern[2:]
		prefix, ok := strings.CutSuffix(host, "."+root)
		return ok && prefix != "" && !strings.Contains(prefix, ".")
	}
	if strings.HasPrefix(pattern, ".") {
		return strings.HasSuffix(host, pattern) && len(host) > len(pattern)
	}
	return host == pattern
}

func normalizeResolver(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !validText(value, maxFieldBytes) || value == "" {
		return "", failure("invalid_dns", 0, "server")
	}
	if value == "system" || value == "system://" {
		return "system", nil
	}
	if ip := net.ParseIP(value); ip != nil {
		return ip.String(), nil
	}
	if !strings.Contains(value, "://") {
		host, port, err := net.SplitHostPort(value)
		n, e := strconv.Atoi(port)
		if err != nil || e != nil || n < 1 || n > 65535 || net.ParseIP(host) == nil {
			return "", failure("invalid_dns", 0, "server")
		}
		return net.JoinHostPort(net.ParseIP(host).String(), port), nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" {
		return "", failure("invalid_dns", 0, "server")
	}
	if !oneOf(u.Scheme, "udp", "tcp", "https", "tls", "quic", "h3") {
		return "", failure("invalid_dns", 0, "server")
	}
	host, err := normalizeServer(u.Hostname())
	if err != nil {
		return "", err
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return "", failure("invalid_dns", 0, "port")
		}
		u.Host = net.JoinHostPort(host, port)
	} else if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	if u.Scheme == "h3" {
		u.Scheme = "https"
		u.Fragment = "h3=true"
	}
	return u.String(), nil
}

// mergeDNSPolicies preserves repeated metadata server order. Each domain's
// general and node policies consume the same complete list.
func mergeDNSPolicies(p *parsedDocument) ([]dnsPolicy, error) {
	result := []dnsPolicy{}
	index := map[string]int{}
	for _, policy := range p.Policies {
		for _, node := range p.Nodes {
			if matchesDomain(policy.Pattern, node.Server) {
				policy.Nodes = true
			}
		}
		if i, ok := index[policy.Pattern]; ok {
			for _, server := range policy.Servers {
				result[i].Servers = appendUnique(result[i].Servers, server)
			}
			result[i].Nodes = result[i].Nodes || policy.Nodes
		} else {
			index[policy.Pattern] = len(result)
			result = append(result, policy)
		}
	}
	return result, nil
}
