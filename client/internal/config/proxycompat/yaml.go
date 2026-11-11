package proxycompat

import "gopkg.in/yaml.v3"

func value(node *yaml.Node, key string) *yaml.Node {
	if node != nil && node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
	}
	return nil
}

func scalar(node *yaml.Node) string {
	if node != nil && node.Kind == yaml.ScalarNode {
		return node.Value
	}
	return ""
}

func size(node *yaml.Node) int {
	if node == nil {
		return 0
	}
	return len(node.Content)
}

func text(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

func put(node *yaml.Node, key string, item *yaml.Node) {
	if old := value(node, key); old != nil {
		*old = *item
		return
	}
	node.Content = append(node.Content, text(key), item)
}

func remove(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func detached(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.AliasNode {
		return detached(node.Alias)
	}
	copy := *node
	copy.Anchor, copy.Alias, copy.Content = "", nil, nil
	for _, child := range node.Content {
		copy.Content = append(copy.Content, detached(child))
	}
	return &copy
}

func filterNames(list *yaml.Node, removed map[string]bool) bool {
	if list == nil || list.Kind != yaml.SequenceNode {
		return false
	}
	kept := list.Content[:0]
	changed := false
	for _, item := range list.Content {
		if removed[scalar(item)] {
			changed = true
		} else {
			kept = append(kept, item)
		}
	}
	list.Content = kept
	return changed
}
