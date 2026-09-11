// Package rulecache preserves HTTP rule-provider files across isolated core
// sessions. Downloads, parsing and expiry remain the responsibility of mihomo.
package rulecache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"

	"gopkg.in/yaml.v3"
)

const sessionDirectory = "rule-provider-cache"
const maxConfigurationBytes = 1 << 20

var errInvalid = errors.New("invalid rule-provider cache input")

type entry struct {
	key      string
	previous string
}

func rewrite(contents []byte) ([]byte, []entry, error) {
	if len(contents) > maxConfigurationBytes {
		return nil, nil, errInvalid
	}
	var document yaml.Node
	if yaml.Unmarshal(contents, &document) != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, errInvalid
	}
	count := 0
	if !validNode(&document, 0, &count) {
		return nil, nil, errInvalid
	}
	providers := field(document.Content[0], "rule-providers")
	if providers == nil {
		return contents, nil, nil
	}
	if providers.Kind != yaml.MappingNode {
		return nil, nil, errInvalid
	}
	var entries []entry
	for i := 0; i < len(providers.Content); i += 2 {
		name, provider := providers.Content[i], providers.Content[i+1]
		kind := field(provider, "type")
		if kind == nil || kind.Value != "http" {
			continue
		}
		var identity map[string]any
		if provider.Decode(&identity) != nil {
			return nil, nil, errInvalid
		}
		// A new generation/path or an interval edit does not change the resource.
		// Include name, URL, headers and all other options to isolate different
		// sources, credentials, formats and provider definitions.
		delete(identity, "path")
		delete(identity, "interval")
		if _, ok := identity["format"]; !ok {
			identity["format"] = "yaml"
		}
		encoded, err := json.Marshal([]any{1, name.Value, identity})
		if err != nil {
			return nil, nil, errInvalid
		}
		digest := sha256.Sum256(encoded)
		item := entry{key: hex.EncodeToString(digest[:])}
		location := field(provider, "path")
		if location != nil {
			item.previous = location.Value
			*location = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: path.Join(sessionDirectory, item.key)}
		} else {
			provider.Content = append(provider.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "path"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: path.Join(sessionDirectory, item.key)})
		}
		entries = append(entries, item)
	}
	if len(entries) == 0 {
		return contents, nil, nil
	}
	result, err := yaml.Marshal(&document)
	if err != nil || len(result) > maxConfigurationBytes {
		return nil, nil, errInvalid
	}
	return result, entries, nil
}

func field(node *yaml.Node, name string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1]
		}
	}
	return nil
}

func validNode(node *yaml.Node, depth int, count *int) bool {
	(*count)++
	if depth > 64 || *count > 50000 || node.Kind == yaml.AliasNode {
		return false
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return false
		}
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if !validNode(child, depth+1, count) {
			return false
		}
	}
	return true
}
