package chainproxy

import (
	"github.com/dlclark/regexp2/v2"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

// Evaluate only top-level nodes already present in the input snapshot. The
// original provider definitions and nested group references stay with mihomo.
type groupMembership struct {
	node         *yaml.Node
	names        []string
	exclude      []*regexp2.Regexp
	excludeTypes []string
	includeAll   bool
	unresolved   bool
}

func compileFilters(text string) ([]*regexp2.Regexp, error) {
	var result []*regexp2.Regexp
	if text == "" {
		return result, nil
	}
	if len(text) > 8192 {
		return nil, failure("group_filter_invalid")
	}
	for _, pattern := range strings.Split(text, "`") {
		re, err := regexp2.Compile(pattern)
		if err != nil {
			return nil, failure("group_filter_invalid")
		}
		re.MatchTimeout = 25 * time.Millisecond
		result = append(result, re)
	}
	return result, nil
}

func regexMatch(filters []*regexp2.Regexp, name string) (bool, error) {
	for _, filter := range filters {
		matched, err := filter.MatchString(name)
		if err != nil {
			return false, failure("group_filter_invalid")
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func membership(group *yaml.Node, order []string, deadline time.Time) (groupMembership, error) {
	m := groupMembership{node: group, includeAll: scalar(value(group, "include-all")) == "true" || scalar(value(group, "include-all-proxies")) == "true"}
	m.unresolved = value(group, "use") != nil || scalar(value(group, "include-all")) == "true" || scalar(value(group, "include-all-providers")) == "true"
	if proxies := value(group, "proxies"); proxies != nil {
		if proxies.Kind != yaml.SequenceNode {
			return m, failure("invalid_selector")
		}
		for _, entry := range proxies.Content {
			if entry.Kind != yaml.ScalarNode || entry.Tag != "!!str" {
				return m, failure("invalid_selector")
			}
			m.names = append(m.names, entry.Value)
		}
	}
	var err error
	m.exclude, err = compileFilters(scalar(value(group, "exclude-filter")))
	if err != nil {
		return m, err
	}
	m.excludeTypes = strings.Split(scalar(value(group, "exclude-type")), "|")
	if m.includeAll {
		filters, err := compileFilters(scalar(value(group, "filter")))
		if err != nil {
			return m, err
		}
		for _, name := range order {
			if time.Now().After(deadline) {
				return m, failure("input_limit")
			}
			matched, err := regexMatch(filters, name)
			if err != nil {
				return m, err
			}
			if len(filters) == 0 || matched {
				m.names = append(m.names, name)
			}
		}
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, name := range m.names {
		if !seen[name] {
			seen[name] = true
			unique = append(unique, name)
		}
	}
	m.names = unique
	return m, nil
}

func (m groupMembership) accepts(name, kind string) (bool, error) {
	excluded, err := regexMatch(m.exclude, name)
	if err != nil || excluded {
		return false, err
	}
	// mihomo uses adapter display names for exclude-type.
	display := map[string]string{"ss": "Shadowsocks", "ssr": "ShadowsocksR", "socks5": "Socks5", "hy2": "Hysteria2"}[kind]
	if display == "" {
		display = kind
	}
	for _, item := range m.excludeTypes {
		if item != "" && strings.EqualFold(item, display) {
			return false, nil
		}
	}
	return true, nil
}

func freezeIncludeAll(m groupMembership) {
	if !m.includeAll {
		return
	}
	if scalar(value(m.node, "include-all")) == "true" {
		put(m.node, "include-all-providers", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	remove(m.node, "include-all")
	remove(m.node, "include-all-proxies")
	names := sequence()
	for _, name := range m.names {
		names.Content = append(names.Content, textNode(name))
	}
	// Preserve the core's fallback for an empty include-all list.
	if len(names.Content) == 0 && !m.unresolved {
		fallback := scalar(value(m.node, "empty-fallback"))
		if fallback == "" {
			fallback = "COMPATIBLE"
		}
		names.Content = append(names.Content, textNode(fallback))
	}
	put(m.node, "proxies", names)
}
