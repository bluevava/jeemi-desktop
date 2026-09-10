package chainproxy

import (
	"gopkg.in/yaml.v3"
	"jeemi/internal/config/document"
	"strings"
)

func value(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func scalar(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}
func textNode(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
func sequence() *yaml.Node         { return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"} }
func mapping() *yaml.Node          { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }
func put(node *yaml.Node, key string, item *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = item
			return
		}
	}
	node.Content = append(node.Content, textNode(key), item)
}
func remove(node *yaml.Node, key string) {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

// Detach aliases before storing a node extracted from a larger document. This
// also avoids anchor-name collisions between independently imported sources.
func detached(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.AliasNode {
		return detached(node.Alias)
	}
	copy := *node
	copy.Anchor = ""
	copy.Alias = nil
	copy.Content = nil
	for _, child := range node.Content {
		copy.Content = append(copy.Content, detached(child))
	}
	return &copy
}

func nodeYAML(node *yaml.Node) string { result, _ := document.EncodeValue(node); return result }
func domainMatches(pattern, host string) bool {
	pattern, host = strings.ToLower(pattern), strings.ToLower(strings.TrimSuffix(host, "."))
	if strings.HasPrefix(pattern, "+.") {
		return host == pattern[2:] || strings.HasSuffix(host, pattern[1:])
	}
	if strings.HasPrefix(pattern, "*.") {
		prefix, ok := strings.CutSuffix(host, pattern[1:])
		return ok && prefix != "" && !strings.Contains(prefix, ".")
	}
	if strings.HasPrefix(pattern, ".") {
		return strings.HasSuffix(host, pattern)
	}
	return pattern == host
}
