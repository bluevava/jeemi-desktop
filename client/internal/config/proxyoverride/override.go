package proxyoverride

import (
	"fmt"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

// Override describes a Jeemi-only bulk change applied to matching objects in
// the subscription's top-level proxies sequence. It is metadata and is never
// written into the final mihomo YAML as a wildcard path.
type Override struct {
	Path            string
	ValueYAML       string
	TargetPath      string
	ApplicableTypes []string
}

// Apply expands proxy overrides against the real subscription nodes while
// preserving each node's identity and required protocol fields.
func Apply(contents []byte, overrides []Override) ([]byte, error) {
	if len(overrides) == 0 {
		return contents, nil
	}
	doc, err := document.Parse(contents)
	if err != nil {
		return nil, fmt.Errorf("parse configuration for proxy overrides: %w", err)
	}
	proxies, found, err := document.Find(document.Root(doc), "/proxies")
	if err != nil {
		return nil, err
	}
	if !found {
		return contents, nil
	}
	if proxies.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("configuration proxies must be a sequence")
	}

	prepared := make([]preparedOverride, 0, len(overrides))
	for _, override := range overrides {
		if override.TargetPath == "" {
			return nil, fmt.Errorf("proxy override %s has no target path", override.Path)
		}
		value, parseErr := document.ParseValue(override.ValueYAML)
		if parseErr != nil {
			return nil, fmt.Errorf("parse proxy override %s: %w", override.Path, parseErr)
		}
		prepared = append(prepared, preparedOverride{
			path:            override.Path,
			targetPath:      override.TargetPath,
			value:           value,
			applicableTypes: normalizedSet(override.ApplicableTypes),
		})
	}

	for index, proxy := range proxies.Content {
		if proxy.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("configuration proxy at index %d must be a mapping", index)
		}
		proxyType := strings.ToLower(strings.TrimSpace(mappingScalar(proxy, "type")))
		for _, override := range prepared {
			if len(override.applicableTypes) > 0 {
				if _, applies := override.applicableTypes[proxyType]; !applies {
					continue
				}
			}
			if err := document.SetMappingPath(proxy, override.targetPath, override.value); err != nil {
				return nil, fmt.Errorf("apply proxy override %s at node %d: %w", override.path, index, err)
			}
		}
	}

	encoded, err := document.Encode(doc)
	if err != nil {
		return nil, fmt.Errorf("encode proxy overrides: %w", err)
	}
	return encoded, nil
}

type preparedOverride struct {
	path            string
	targetPath      string
	value           *yaml.Node
	applicableTypes map[string]struct{}
}

func normalizedSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func mappingScalar(mapping *yaml.Node, key string) string {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key && mapping.Content[index+1].Kind == yaml.ScalarNode {
			return mapping.Content[index+1].Value
		}
	}
	return ""
}
