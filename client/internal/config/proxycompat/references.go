package proxycompat

import (
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

func repairReferences(root *yaml.Node, removed, providers, changedProviders map[string]bool) {
	groups := value(root, "proxy-groups")
	if groups != nil && groups.Kind == yaml.SequenceNode {
		for _, group := range groups.Content {
			if group.Kind != yaml.MappingNode {
				continue
			}
			changed := filterNames(value(group, "proxies"), removed)
			providerChanged := referencesAny(value(group, "use"), changedProviders)
			changed = filterNames(value(group, "use"), providers) || changed
			affected := changed || providerChanged ||
				len(removed) > 0 && enabled(group, "include-all", "include-all-proxies") ||
				len(changedProviders) > 0 && enabled(group, "include-all", "include-all-providers")
			rejectReference(value(group, "empty-fallback"), removed)
			if affected && scalar(value(group, "empty-fallback")) == "" {
				// Dynamic name/type filters can also leave the group empty. Let
				// mihomo evaluate them, but never use its implicit DIRECT fallback.
				put(group, "empty-fallback", text("REJECT"))
			}
			if removed[scalar(value(group, "default-selected"))] {
				remove(group, "default-selected")
			}
			if affected && scalar(value(group, "type")) == "relay" {
				// A partially removed relay must not become a shorter route.
				put(group, "type", text("select"))
				put(group, "empty-fallback", text("REJECT"))
				put(group, "proxies", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{text("REJECT")}})
				for _, key := range []string{"use", "include-all", "include-all-proxies", "include-all-providers", "default-selected"} {
					remove(group, key)
				}
				continue
			}
			includesProxies := enabled(group, "include-all", "include-all-proxies") && size(value(root, "proxies")) > 0
			includesProviders := enabled(group, "include-all", "include-all-providers") && size(value(root, "proxy-providers")) > 0
			if affected && size(value(group, "proxies")) == 0 && size(value(group, "use")) == 0 && !includesProxies && !includesProviders {
				put(group, "proxies", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{text("REJECT")}})
			}
		}
	}
	repairRules(value(root, "rules"), removed)
	if subRules := value(root, "sub-rules"); subRules != nil && subRules.Kind == yaml.MappingNode {
		for i := 1; i < len(subRules.Content); i += 2 {
			repairRules(subRules.Content[i], removed)
		}
	}
	for _, key := range []string{"proxy-providers", "rule-providers"} {
		if entries := value(root, key); entries != nil && entries.Kind == yaml.MappingNode {
			for i := 1; i < len(entries.Content); i += 2 {
				entry := entries.Content[i]
				rejectReference(value(entry, "proxy"), removed)
				rejectReference(value(value(entry, "override"), "dialer-proxy"), removed)
			}
		}
	}
	for _, key := range []string{"listeners", "tunnels"} {
		if entries := value(root, key); entries != nil && entries.Kind == yaml.SequenceNode {
			for _, entry := range entries.Content {
				rejectReference(value(entry, "proxy"), removed)
			}
		}
	}
	dns := value(root, "dns")
	for _, key := range []string{"nameserver", "fallback", "default-nameserver", "direct-nameserver", "proxy-server-nameserver", "nameserver-policy", "proxy-server-nameserver-policy"} {
		repairResolvers(value(dns, key), removed)
	}
}

func referencesAny(list *yaml.Node, names map[string]bool) bool {
	if list != nil && list.Kind == yaml.SequenceNode {
		for _, item := range list.Content {
			if names[scalar(item)] {
				return true
			}
		}
	}
	return false
}

func enabled(node *yaml.Node, keys ...string) bool {
	for _, key := range keys {
		if strings.EqualFold(scalar(value(node, key)), "true") {
			return true
		}
	}
	return false
}

func rejectReference(node *yaml.Node, removed map[string]bool) {
	if node != nil && node.Kind == yaml.ScalarNode && removed[node.Value] {
		node.Value, node.Tag = "REJECT", "!!str"
	}
}

func repairRules(rules *yaml.Node, removed map[string]bool) {
	if rules == nil || rules.Kind != yaml.SequenceNode {
		return
	}
	for _, rule := range rules.Content {
		if rule.Kind != yaml.ScalarNode || rule.Tag != "!!str" {
			continue
		}
		// The outbound is the final comma-separated token (before no-resolve),
		// including logical rules with nested, comma-separated conditions.
		body, suffix := rule.Value, ""
		if i := strings.LastIndexByte(body, ','); i >= 0 && strings.EqualFold(strings.TrimSpace(body[i+1:]), "no-resolve") {
			suffix, body = body[i:], body[:i]
		}
		kind, _, _ := strings.Cut(body, ",")
		if strings.EqualFold(strings.TrimSpace(kind), "SUB-RULE") {
			continue // Its final token names a sub-rule, not an outbound.
		}
		if i := strings.LastIndexByte(body, ','); i >= 0 && removed[strings.TrimSpace(body[i+1:])] {
			rule.Value = body[:i+1] + "REJECT" + suffix
		}
	}
}

func repairResolvers(node *yaml.Node, removed map[string]bool) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			repairResolvers(node.Content[i], removed)
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			repairResolvers(child, removed)
		}
	case yaml.ScalarNode:
		base, fragment, ok := strings.Cut(node.Value, "#")
		if !ok {
			return
		}
		parts := strings.Split(fragment, "&")
		for i, part := range parts {
			name, err := url.PathUnescape(part)
			if err == nil && removed[name] {
				parts[i] = "REJECT"
			}
		}
		node.Value = base + "#" + strings.Join(parts, "&")
	}
}

func ensureRejectFallback(root *yaml.Node) {
	rules := value(root, "rules")
	if rules == nil {
		rules = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		put(root, "rules", rules)
	}
	if rules.Kind != yaml.SequenceNode {
		return
	}
	for _, rule := range rules.Content {
		kind, _, _ := strings.Cut(rule.Value, ",")
		if strings.EqualFold(strings.TrimSpace(kind), "MATCH") || strings.EqualFold(strings.TrimSpace(kind), "FINAL") {
			return
		}
	}
	rules.Content = append(rules.Content, text("MATCH,REJECT"))
}
