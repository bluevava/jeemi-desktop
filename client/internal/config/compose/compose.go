// 作者：bluevava 及贡献者
// 开源仓库：https://github.com/bluevava/jeemi-android

package compose

import (
	"fmt"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

// Compose applies a sparse local overlay to an immutable subscription YAML.
// The function has no filesystem, process, or network side effects.
func Compose(subscriptionYAML, overlayYAML []byte, plan []Rule) (Result, error) {
	subscription, err := document.Parse(subscriptionYAML)
	if err != nil {
		return Result{}, fmt.Errorf("parse subscription configuration: %w", err)
	}
	overlay, err := document.Parse(overlayYAML)
	if err != nil {
		return Result{}, fmt.Errorf("parse local configuration: %w", err)
	}
	if err := ValidatePlan(document.Root(overlay), plan); err != nil {
		return Result{}, err
	}

	result := document.Clone(subscription)
	for _, rule := range plan {
		local, found, findErr := document.Find(document.Root(overlay), rule.Path)
		if findErr != nil || !found {
			return Result{}, fmt.Errorf("resolve local value at %s", rule.Path)
		}
		base, baseFound, findErr := document.Find(document.Root(result), rule.Path)
		if findErr != nil {
			return Result{}, findErr
		}
		if !baseFound {
			if err := document.SetMappingPath(document.Root(result), rule.Path, local); err != nil {
				return Result{}, err
			}
			continue
		}

		combined, err := combineNodes(rule, base, local)
		if err != nil {
			return Result{}, err
		}
		if err := document.SetMappingPath(document.Root(result), rule.Path, combined); err != nil {
			return Result{}, err
		}
	}

	encoded, err := document.Encode(result)
	if err != nil {
		return Result{}, err
	}
	return Result{YAML: string(encoded)}, nil
}

func combineNodes(rule Rule, base, local *yaml.Node) (*yaml.Node, error) {
	switch rule.Strategy {
	case StrategyReplace:
		return document.Clone(local), nil
	case StrategyMerge:
		if base.Kind != yaml.MappingNode || local.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("merge requires mappings at %s", rule.Path)
		}
		return mergeMappings(base, local), nil
	case StrategyPrepend:
		return mergeSequences(rule.Path, local, base)
	case StrategyAppend:
		return mergeSequences(rule.Path, base, local)
	case StrategyAppendBeforeTerminal:
		return appendBeforeTerminal(rule.Path, base, local)
	case StrategyMergeByName:
		return mergeNamed(rule, base, local)
	default:
		return nil, fmt.Errorf("unsupported merge strategy at %s", rule.Path)
	}
}

func mergeMappings(base, local *yaml.Node) *yaml.Node {
	result := document.Clone(base)
	for index := 0; index < len(local.Content); index += 2 {
		key := local.Content[index]
		localValue := local.Content[index+1]
		resultIndex := mappingValueIndex(result, key.Value)
		if resultIndex < 0 {
			result.Content = append(result.Content, document.Clone(key), document.Clone(localValue))
			continue
		}
		baseValue := result.Content[resultIndex]
		if baseValue.Kind == yaml.MappingNode && localValue.Kind == yaml.MappingNode {
			result.Content[resultIndex] = mergeMappings(baseValue, localValue)
		} else {
			result.Content[resultIndex] = document.Clone(localValue)
		}
	}
	return result
}

func mergeSequences(path string, first, second *yaml.Node) (*yaml.Node, error) {
	if first.Kind != yaml.SequenceNode || second.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("sequence merge requires arrays at %s", path)
	}
	result := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seen := make(map[string]struct{}, len(first.Content)+len(second.Content))
	for _, source := range [][]*yaml.Node{first.Content, second.Content} {
		for _, value := range source {
			identity, err := document.EncodeValue(value)
			if err != nil {
				return nil, err
			}
			if _, exists := seen[identity]; exists {
				continue
			}
			seen[identity] = struct{}{}
			result.Content = append(result.Content, document.Clone(value))
		}
	}
	return result, nil
}

func appendBeforeTerminal(path string, base, local *yaml.Node) (*yaml.Node, error) {
	if base.Kind != yaml.SequenceNode || local.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("rule merge requires arrays at %s", path)
	}
	terminal := len(base.Content)
	for index, rule := range base.Content {
		if isTerminalRule(rule) {
			terminal = index
			break
		}
	}
	result := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range base.Content[:terminal] {
		result.Content = append(result.Content, document.Clone(value))
	}
	for _, value := range local.Content {
		result.Content = append(result.Content, document.Clone(value))
	}
	for _, value := range base.Content[terminal:] {
		result.Content = append(result.Content, document.Clone(value))
	}
	return result, nil
}

func isTerminalRule(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}
	kind := strings.ToUpper(strings.TrimSpace(strings.SplitN(node.Value, ",", 2)[0]))
	return kind == "MATCH" || kind == "FINAL"
}

func mergeNamed(rule Rule, base, local *yaml.Node) (*yaml.Node, error) {
	policy := rule.ConflictPolicy
	if policy == "" {
		policy = ConflictError
	}
	if base.Kind == yaml.MappingNode && local.Kind == yaml.MappingNode {
		return mergeNamedMapping(rule.Path, policy, base, local)
	}
	if base.Kind == yaml.SequenceNode && local.Kind == yaml.SequenceNode {
		return mergeNamedSequence(rule.Path, policy, base, local)
	}
	return nil, fmt.Errorf("named merge requires matching collections at %s", rule.Path)
}

func mergeNamedMapping(path string, policy ConflictPolicy, base, local *yaml.Node) (*yaml.Node, error) {
	result := document.Clone(base)
	for index := 0; index < len(local.Content); index += 2 {
		key := local.Content[index]
		value := local.Content[index+1]
		resultIndex := mappingValueIndex(result, key.Value)
		if resultIndex >= 0 {
			if policy != ConflictUseLocal {
				return nil, &Conflict{Path: path, Name: key.Value}
			}
			result.Content[resultIndex] = document.Clone(value)
			continue
		}
		result.Content = append(result.Content, document.Clone(key), document.Clone(value))
	}
	return result, nil
}

func mergeNamedSequence(path string, policy ConflictPolicy, base, local *yaml.Node) (*yaml.Node, error) {
	result := document.Clone(base)
	indices, err := namedSequenceIndices(path, result)
	if err != nil {
		return nil, err
	}
	localSeen := make(map[string]struct{}, len(local.Content))
	for _, value := range local.Content {
		name, err := namedItemName(path, value)
		if err != nil {
			return nil, err
		}
		if _, duplicate := localSeen[name]; duplicate {
			return nil, fmt.Errorf("duplicate local named item %q at %s", name, path)
		}
		localSeen[name] = struct{}{}
		if index, exists := indices[name]; exists {
			if policy != ConflictUseLocal {
				return nil, &Conflict{Path: path, Name: name}
			}
			result.Content[index] = document.Clone(value)
			continue
		}
		indices[name] = len(result.Content)
		result.Content = append(result.Content, document.Clone(value))
	}
	return result, nil
}

func namedSequenceIndices(path string, sequence *yaml.Node) (map[string]int, error) {
	indices := make(map[string]int, len(sequence.Content))
	for index, value := range sequence.Content {
		name, err := namedItemName(path, value)
		if err != nil {
			return nil, err
		}
		if _, duplicate := indices[name]; duplicate {
			return nil, fmt.Errorf("duplicate subscription named item %q at %s", name, path)
		}
		indices[name] = index
	}
	return indices, nil
}

func namedItemName(path string, value *yaml.Node) (string, error) {
	if value == nil || value.Kind != yaml.MappingNode {
		return "", fmt.Errorf("named collection item must be a mapping at %s", path)
	}
	index := mappingValueIndex(value, "name")
	if index < 0 || value.Content[index].Kind != yaml.ScalarNode || strings.TrimSpace(value.Content[index].Value) == "" {
		return "", fmt.Errorf("named collection item has no name at %s", path)
	}
	return value.Content[index].Value, nil
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return index + 1
		}
	}
	return -1
}
