package subscriptionformat

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

type surgeEntry struct {
	value string
	line  int
}

func parseSurge(text string) (parsedDocument, error) {
	p := parsedDocument{Hosts: map[string][]string{}}
	lines := strings.Split(text, "\n")
	if len(lines) > maxLines {
		return p, failure("size_limit", 0, "")
	}
	general := map[string]surgeEntry{}
	section, managed := "", ""
	groups, rules, proxyNames := []string{}, []string{}, []string{}
	for index, line := range lines {
		line = strings.TrimSpace(line)
		location := index + 1
		if len(line) > maxLineBytes {
			return p, failure("size_limit", location, "")
		}
		if strings.HasPrefix(line, "#!MANAGED-CONFIG ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				managed = fields[1]
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		line = stripSurgeComment(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		switch section {
		case "general":
			key, value, ok := surgeAssignment(line)
			if !ok {
				return p, failure("invalid_surge", location, "")
			}
			key = strings.ToLower(key)
			if _, duplicate := general[key]; duplicate {
				return p, failure("invalid_surge", location, "")
			}
			general[key] = surgeEntry{value, location}
		case "host":
			if err := parseSurgeHost(&p, line, location); err != nil {
				return p, err
			}
		case "proxy":
			name, value, ok := surgeAssignment(line)
			if !ok {
				return p, failure("invalid_surge", location, "")
			}
			name, ok = surgeValue(name)
			if !ok || !validText(name, 1024) || name == "" {
				return p, failure("invalid_field", location, "name")
			}
			proxyNames = append(proxyNames, name)
			n, unsupported, err := parseSurgeProxy(name, value, location)
			if err != nil {
				return p, err
			}
			if unsupported != "" {
				p.skip(unsupported, location, "")
			} else {
				p.Nodes = append(p.Nodes, n)
			}
		case "proxy group":
			groups = append(groups, line)
		case "rule":
			rules = append(rules, line)
		case "":
			return p, failure("invalid_surge", location, "")
		default: // Routing, scripts, MITM and runtime settings are not imported.
		}
	}
	plain, err := surgeResolvers(general["dns-server"])
	if err != nil {
		return p, err
	}
	encrypted, err := surgeResolvers(general["encrypted-dns-server"])
	if err != nil {
		return p, err
	}
	if len(plain) == 0 {
		plain = []string{"system"}
	}
	for _, server := range plain {
		if strings.Contains(server, "://") && !strings.HasPrefix(server, "udp://") {
			encrypted = appendUnique(encrypted, server)
		}
	}
	if value, ok := general["encrypted-dns-skip-cert-verification"]; ok {
		v, valid := normalizeField(value.value, "bool")
		if !valid {
			return p, failure("invalid_dns", value.line, "")
		}
		if v.(bool) {
			return p, failure("unsupported_dns_options", value.line, "")
		}
	}
	if value, ok := general["encrypted-dns-follow-outbound-mode"]; ok {
		v, valid := normalizeField(value.value, "bool")
		if !valid {
			return p, failure("invalid_dns", value.line, "")
		}
		if v.(bool) {
			return p, failure("unsupported_dns_options", value.line, "")
		}
	}
	// This producer signature is a format hint only, never a trust decision.
	if isJeeyioSurge(managed, groups, rules, proxyNames, &p, encrypted) {
		private := map[string]bool{}
		for i := range p.Policies {
			p.Policies[i].Nodes = true
			for _, server := range p.Policies[i].Servers {
				private[server] = true
			}
		}
		filtered := []string{}
		for _, server := range encrypted {
			if !private[server] {
				filtered = append(filtered, server)
			}
		}
		encrypted = filtered
	} else if len(p.Policies) > 0 && len(encrypted) > 0 {
		p.Diagnostics = append(p.Diagnostics, Diagnostic{Code: "surge_dns_scope_preserved"})
	}
	if len(encrypted) > 0 {
		p.Nameservers = encrypted
		for _, server := range plain {
			host := server
			if h, _, e := net.SplitHostPort(server); e == nil {
				host = h
			}
			if net.ParseIP(host) != nil {
				p.Bootstrap = appendUnique(p.Bootstrap, server)
			}
		}
	} else {
		p.Nameservers = plain
	}
	return p, nil
}

func surgeResolvers(entry surgeEntry) ([]string, error) {
	if entry.value == "" {
		return nil, nil
	}
	parts, ok := splitSurge(entry.value, ',')
	if !ok {
		return nil, failure("invalid_dns", entry.line, "")
	}
	servers := []string{}
	for _, part := range parts {
		value, ok := surgeValue(part)
		if !ok {
			return nil, failure("invalid_dns", entry.line, "")
		}
		server, err := normalizeResolver(value)
		if err != nil {
			return nil, failure("invalid_dns", entry.line, "server")
		}
		servers = appendUnique(servers, server)
	}
	return servers, nil
}

func parseSurgeHost(p *parsedDocument, line string, location int) error {
	key, value, ok := surgeAssignment(line)
	if !ok {
		return failure("invalid_dns", location, "")
	}
	key, ok = surgeValue(key)
	if !ok {
		return failure("invalid_dns", location, "domain")
	}
	pattern, err := normalizePattern(key, true)
	if err != nil {
		return failure("unsupported_host_mapping", location, "domain")
	}
	if strings.HasPrefix(value, "server:") {
		servers, err := surgeResolvers(surgeEntry{strings.TrimPrefix(value, "server:"), location})
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			return failure("invalid_dns", location, "server")
		}
		p.Policies = append(p.Policies, dnsPolicy{Pattern: pattern, Servers: servers, Line: location})
		return nil
	}
	parts, ok := splitSurge(value, ',')
	if !ok {
		return failure("invalid_dns", location, "")
	}
	if _, exists := p.Hosts[pattern]; exists {
		return failure("invalid_dns", location, "domain")
	}
	for _, part := range parts {
		v, ok := surgeValue(part)
		ip := net.ParseIP(v)
		if !ok || ip == nil {
			return failure("unsupported_host_mapping", location, "")
		}
		p.Hosts[pattern] = appendUnique(p.Hosts[pattern], ip.String())
	}
	return nil
}

func parseSurgeProxy(name, value string, line int) (proxyNode, string, error) {
	n := proxyNode{Name: name, Line: line}
	parts, ok := splitSurge(value, ',')
	if !ok || len(parts) == 0 {
		return n, "", failure("invalid_surge", line, "")
	}
	for i := 0; i < len(parts) && i < 3; i++ {
		parts[i], ok = surgeValue(parts[i])
		if !ok {
			return n, "", failure("invalid_surge", line, "")
		}
	}
	switch strings.ToLower(parts[0]) {
	case "ss", "socks5", "anytls":
		n.Protocol = strings.ToLower(parts[0])
	case "tuic-v5":
		n.Protocol = "tuic"
	default:
		return n, "unsupported_protocol", nil
	}
	if len(parts) < 3 {
		return n, "", failure("invalid_surge", line, "")
	}
	var err error
	n.Server, err = normalizeServer(strings.Trim(parts[1], "[]"))
	if err != nil {
		return n, "", failure("invalid_field", line, "server")
	}
	n.Port, err = strconv.Atoi(parts[2])
	if err != nil || n.Port < 1 || n.Port > 65535 {
		return n, "", failure("invalid_field", line, "port")
	}
	values := url.Values{}
	positionals := []string{}
	for _, part := range parts[3:] {
		key, value, assignment := surgeAssignment(part)
		if !assignment {
			v, ok := surgeValue(part)
			if !ok {
				return n, "", failure("invalid_surge", line, "")
			}
			positionals = append(positionals, v)
			continue
		}
		v, ok := surgeValue(value)
		if !ok {
			return n, "", failure("invalid_surge", line, "")
		}
		values.Add(strings.ToLower(key), v)
	}
	if n.Protocol == "socks5" && len(positionals) == 2 {
		values.Add("username", positionals[0])
		values.Add("password", positionals[1])
	} else if len(positionals) > 0 {
		return n, "unsupported_node_fields", nil
	}
	// Consume the protocol's authentication fields without exposing values in
	// errors. TLS aliases then go through the same registry as URI imports.
	take := func(key string) string {
		v := values[key]
		delete(values, key)
		if len(v) == 0 {
			return ""
		}
		for _, item := range v {
			if item != v[0] {
				err = failure("conflicting_alias", line, "credentials")
			}
		}
		return v[0]
	}
	n.Password = take("password")
	switch n.Protocol {
	case "ss":
		n.Cipher = take("encrypt-method")
		if n.Cipher == "" {
			err = failure("invalid_field", line, "cipher")
		}
	case "socks5":
		n.Username = take("username")
		if !validCredential(n.Username) {
			err = failure("invalid_field", line, "credentials")
		}
	case "tuic":
		n.UUID = take("uuid")
		if !validUUID(n.UUID) {
			err = failure("invalid_field", line, "credentials")
		}
	}
	if err != nil {
		return n, "", err
	}
	if !validCredential(n.Password) {
		return n, "", failure("invalid_field", line, "credentials")
	}
	r := reader(values, line)
	n.UDP = r.boolean("udp")
	if n.Protocol == "anytls" || n.Protocol == "tuic" {
		n.TLS = readTLS(r)
		n.TLS.Enabled = true
	}
	if n.Protocol == "anytls" && n.UDP == nil {
		n.UDP = boolValue(true)
	}
	if r.err != nil {
		return n, "", r.err
	}
	if r.unknown() {
		return n, "unsupported_node_fields", nil
	}
	if reason := unsupportedNodeFeature(n); reason != "" {
		return n, reason, nil
	}
	if err := validateNode(n); err != nil {
		return n, "", err
	}
	return n, "", nil
}

func isJeeyioSurge(managed string, groups, rules, names []string, p *parsedDocument, encrypted []string) bool {
	u, err := url.Parse(managed)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Query().Get("type") != "surge" || !strings.HasPrefix(u.Path, "/api/subscribe/") || strings.Contains(strings.TrimPrefix(u.Path, "/api/subscribe/"), "/") {
		return false
	}
	if len(groups) != 1 || len(rules) != 1 || len(p.Policies) == 0 || len(encrypted) == 0 {
		return false
	}
	name, value, ok := surgeAssignment(groups[0])
	if !ok {
		return false
	}
	name, ok = surgeValue(name)
	if !ok {
		return false
	}
	parts, ok := splitSurge(value, ',')
	if !ok || len(parts) != len(names)+1 || parts[0] != "select" {
		return false
	}
	for i, n := range names {
		v, ok := surgeValue(parts[i+1])
		if !ok || v != n {
			return false
		}
	}
	if rules[0] != "FINAL,"+name {
		return false
	}
	for _, policy := range p.Policies {
		if !strings.HasPrefix(policy.Pattern, ".") {
			return false
		}
		matched := false
		for _, node := range p.Nodes {
			if matchesDomain(policy.Pattern, node.Server) {
				matched = true
			}
		}
		if !matched {
			return false
		}
		for _, server := range policy.Servers {
			if !oneOf(server, encrypted...) {
				return false
			}
		}
	}
	return true
}
