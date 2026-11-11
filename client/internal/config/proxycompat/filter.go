// Package proxycompat removes unsupported outbounds from derived runtime
// configurations. It never changes subscription revisions or local resources.
package proxycompat

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
)

// Protocol names and Snell options are checked against the official
// mihomo v1.19.33 adapter/parser.go and adapter/outbound/snell.go.
// Keep this separate from the narrower URI conversion field catalogue.
var supportedProtocols = strings.Fields("ss ssr socks5 http vmess vless snell trojan hysteria hysteria2 wireguard tuic shadowquic gost-relay direct dns reject rematch ssh mieru anytls sudoku masque trusttunnel openvpn tailscale zerotier easytier")

func unsupported(node *yaml.Node) bool {
	typeNode := value(node, "type")
	if typeNode == nil || typeNode.Kind != yaml.ScalarNode || typeNode.Tag != "!!str" || typeNode.Value == "" {
		return false // Malformed configurations still go through normal validation.
	}
	kind := typeNode.Value
	supported := false
	for _, name := range supportedProtocols {
		supported = supported || kind == name
	}
	if !supported {
		return true
	}
	if kind == "snell" {
		if version := value(node, "version"); version != nil {
			n, err := strconv.Atoi(version.Value)
			if version.Kind == yaml.ScalarNode && version.Tag == "!!int" {
				err = version.Decode(&n)
			}
			if err == nil && (n < 0 || n > 5) {
				return true
			}
		}
		return scalar(value(node, "userkey")) != "" || scalar(value(node, "mode")) != ""
	}
	return false
}

type collection struct {
	nodes    *yaml.Node
	provider string // Empty for the globally named proxies collection.
}

type outbound struct {
	node   *yaml.Node
	global bool
}

// Apply only filters recognized unsupported capabilities. Syntax errors,
// invalid credentials and other core validation failures are not swallowed.
func Apply(contents []byte) ([]byte, error) {
	parsed, err := document.Parse(contents)
	if err != nil {
		return nil, err
	}
	// Expanding aliases prevents dangling anchors or changing an unrelated alias
	// when a referenced node/list is removed. The parser already bounds expansion.
	parsed = detached(parsed)
	root := document.Root(parsed)
	collections := []collection{{nodes: value(root, "proxies")}}
	providers := value(root, "proxy-providers")
	if providers != nil && providers.Kind == yaml.MappingNode {
		for i := 0; i < len(providers.Content); i += 2 {
			provider := providers.Content[i+1]
			if scalar(value(provider, "type")) == "inline" {
				collections = append(collections, collection{value(provider, "payload"), providers.Content[i].Value})
			}
		}
	}
	removed := map[*yaml.Node]bool{}
	removedNames := map[string]bool{}
	dependants := map[string][]outbound{}
	queue := []string{}
	removeNode := func(item outbound) {
		if removed[item.node] {
			return
		}
		removed[item.node] = true
		if name := scalar(value(item.node, "name")); item.global && name != "" {
			removedNames[name] = true
			queue = append(queue, name)
		}
	}
	for _, list := range collections {
		if list.nodes == nil || list.nodes.Kind != yaml.SequenceNode {
			continue
		}
		for _, node := range list.nodes.Content {
			item := outbound{node, list.provider == ""}
			if unsupported(node) {
				removeNode(item)
			}
			if dialer := scalar(value(node, "dialer-proxy")); dialer != "" {
				dependants[dialer] = append(dependants[dialer], item)
			}
		}
	}
	if len(removed) == 0 {
		return bytes.Clone(contents), nil
	}
	// Dropping only dialer-proxy would silently bypass a requested relay.
	for i := 0; i < len(queue); i++ {
		for _, item := range dependants[queue[i]] {
			removeNode(item)
		}
	}
	if err := validateRemovedNames(root, removedNames); err != nil {
		return nil, err
	}
	removedProviders := map[string]bool{}
	changedProviders := map[string]bool{}
	remaining := 0
	for _, list := range collections {
		if list.nodes == nil || list.nodes.Kind != yaml.SequenceNode {
			continue
		}
		before := len(list.nodes.Content)
		kept := list.nodes.Content[:0]
		for _, node := range list.nodes.Content {
			if !removed[node] {
				kept = append(kept, node)
			}
		}
		list.nodes.Content = kept
		remaining += len(kept)
		if list.provider != "" && before != len(kept) {
			changedProviders[list.provider] = true
		}
		if list.provider != "" && before > 0 && len(kept) == 0 {
			removedProviders[list.provider] = true
			remove(providers, list.provider)
		}
	}
	repairReferences(root, removedNames, removedProviders, changedProviders)
	if remaining == 0 && size(providers) == 0 {
		ensureRejectFallback(root)
	}
	encoded, err := document.Encode(parsed)
	if err != nil {
		return nil, err
	}
	if _, err := document.Parse(encoded); err != nil {
		return nil, fmt.Errorf("filtered configuration exceeds YAML limits")
	}
	return encoded, nil
}

func validateRemovedNames(root *yaml.Node, removed map[string]bool) error {
	seen := map[string]bool{"DIRECT": true, "REJECT": true, "REJECT-DROP": true, "PASS": true, "COMPATIBLE": true, "GLOBAL": true}
	for _, key := range []string{"proxies", "proxy-groups"} {
		list := value(root, key)
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		for _, node := range list.Content {
			name := scalar(value(node, "name"))
			if seen[name] && removed[name] {
				return fmt.Errorf("unsupported proxy has an ambiguous name")
			}
			seen[name] = true
		}
	}
	return nil
}
