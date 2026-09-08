package document

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// PointerSegments parses an RFC 6901 style path used by merge-plan.json.
func PointerSegments(pointer string) ([]string, error) {
	if pointer == "" || pointer[0] != '/' {
		return nil, fmt.Errorf("configuration path must start with /")
	}
	parts := strings.Split(pointer[1:], "/")
	segments := make([]string, len(parts))
	for index, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("configuration path contains an empty segment")
		}
		var builder strings.Builder
		for offset := 0; offset < len(part); offset++ {
			if part[offset] != '~' {
				builder.WriteByte(part[offset])
				continue
			}
			if offset+1 >= len(part) || (part[offset+1] != '0' && part[offset+1] != '1') {
				return nil, fmt.Errorf("configuration path contains an invalid escape")
			}
			offset++
			if part[offset] == '0' {
				builder.WriteByte('~')
			} else {
				builder.WriteByte('/')
			}
		}
		segments[index] = builder.String()
	}
	return segments, nil
}

// Find resolves a path below a mapping document root.
func Find(root *yaml.Node, pointer string) (*yaml.Node, bool, error) {
	segments, err := PointerSegments(pointer)
	if err != nil {
		return nil, false, err
	}
	current := root
	for _, segment := range segments {
		if current == nil {
			return nil, false, nil
		}
		switch current.Kind {
		case yaml.MappingNode:
			value, found := mappingValue(current, segment)
			if !found {
				return nil, false, nil
			}
			current = value
		case yaml.SequenceNode:
			index, parseErr := strconv.Atoi(segment)
			if parseErr != nil || index < 0 || index >= len(current.Content) {
				return nil, false, nil
			}
			current = current.Content[index]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}

// SetMappingPath writes a value through mapping-only path segments, creating
// missing mappings but refusing to replace an existing scalar parent.
func SetMappingPath(root *yaml.Node, pointer string, value *yaml.Node) error {
	if root == nil || root.Kind != yaml.MappingNode {
		return fmt.Errorf("configuration root must be a mapping")
	}
	segments, err := PointerSegments(pointer)
	if err != nil {
		return err
	}
	current := root
	for index, segment := range segments {
		last := index == len(segments)-1
		if current.Kind != yaml.MappingNode {
			return fmt.Errorf("configuration path parent is not a mapping at %s", pointer)
		}
		valueIndex := mappingValueIndex(current, segment)
		if last {
			if valueIndex >= 0 {
				current.Content[valueIndex] = Clone(value)
			} else {
				current.Content = append(current.Content, scalarKey(segment), Clone(value))
			}
			return nil
		}
		if valueIndex < 0 {
			next := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content, scalarKey(segment), next)
			current = next
			continue
		}
		current = current.Content[valueIndex]
	}
	return nil
}

// DeleteMappingPath removes a value through mapping-only path segments. Empty
// mappings created solely for the removed value are pruned. Missing paths are
// treated as already deleted.
func DeleteMappingPath(root *yaml.Node, pointer string) error {
	if root == nil || root.Kind != yaml.MappingNode {
		return fmt.Errorf("configuration root must be a mapping")
	}
	segments, err := PointerSegments(pointer)
	if err != nil {
		return err
	}
	parents := make([]*yaml.Node, 0, len(segments))
	indices := make([]int, 0, len(segments))
	current := root
	for _, segment := range segments {
		if current.Kind != yaml.MappingNode {
			return fmt.Errorf("configuration path parent is not a mapping at %s", pointer)
		}
		valueIndex := mappingValueIndex(current, segment)
		if valueIndex < 0 {
			return nil
		}
		parents = append(parents, current)
		indices = append(indices, valueIndex)
		current = current.Content[valueIndex]
	}

	for index := len(parents) - 1; index >= 0; index-- {
		parent := parents[index]
		valueIndex := indices[index]
		parent.Content = append(parent.Content[:valueIndex-1], parent.Content[valueIndex+1:]...)
		if len(parent.Content) > 0 || index == 0 {
			break
		}
	}
	return nil
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	index := mappingValueIndex(mapping, key)
	if index < 0 {
		return nil, false
	}
	return mapping.Content[index], true
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return -1
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return index + 1
		}
	}
	return -1
}

func scalarKey(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
