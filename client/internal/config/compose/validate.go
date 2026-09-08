package compose

import (
	"fmt"
	"sort"
	"strings"

	"jeemi/internal/config/document"

	"gopkg.in/yaml.v3"
)

var namedSequencePaths = map[string]struct{}{
	"/listeners":    {},
	"/proxies":      {},
	"/proxy-groups": {},
}

var namedMappingPaths = map[string]struct{}{
	"/proxy-providers": {},
	"/rule-providers":  {},
	"/sub-rules":       {},
}

func ValidatePlan(overlayRoot *yaml.Node, plan []Rule) error {
	if overlayRoot == nil || overlayRoot.Kind != yaml.MappingNode {
		return fmt.Errorf("local configuration root must be a mapping")
	}
	if len(plan) == 0 && len(overlayRoot.Content) != 0 {
		return fmt.Errorf("local configuration has values without a merge plan")
	}

	paths := make([]string, 0, len(plan))
	seen := make(map[string]struct{}, len(plan))
	for _, rule := range plan {
		if _, err := document.PointerSegments(rule.Path); err != nil {
			return fmt.Errorf("invalid merge path %q: %w", rule.Path, err)
		}
		if _, exists := seen[rule.Path]; exists {
			return fmt.Errorf("duplicate merge path %s", rule.Path)
		}
		seen[rule.Path] = struct{}{}
		paths = append(paths, rule.Path)

		node, found, err := document.Find(overlayRoot, rule.Path)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("merge path %s has no local value", rule.Path)
		}
		if !strategyAllowed(rule.Path, node, rule.Strategy) {
			return fmt.Errorf("merge strategy %s is not valid for %s", rule.Strategy, rule.Path)
		}
		if rule.ConflictPolicy != "" && rule.ConflictPolicy != ConflictError && rule.ConflictPolicy != ConflictUseLocal {
			return fmt.Errorf("invalid conflict policy for %s", rule.Path)
		}
		if rule.Strategy != StrategyMergeByName && rule.ConflictPolicy == ConflictUseLocal {
			return fmt.Errorf("local conflict resolution is only valid for named collections at %s", rule.Path)
		}
	}

	sort.Strings(paths)
	for index := 1; index < len(paths); index++ {
		if strings.HasPrefix(paths[index], paths[index-1]+"/") {
			return fmt.Errorf("overlapping merge paths %s and %s", paths[index-1], paths[index])
		}
	}
	if err := validateOverlayCoverage(overlayRoot, "", seen); err != nil {
		return err
	}
	return nil
}

func validateOverlayCoverage(node *yaml.Node, path string, planPaths map[string]struct{}) error {
	if _, covered := planPaths[path]; covered {
		return nil
	}
	if node.Kind != yaml.MappingNode || len(node.Content) == 0 {
		if path == "" && node.Kind == yaml.MappingNode && len(node.Content) == 0 {
			return nil
		}
		return fmt.Errorf("local value at %s is not declared in the merge plan", displayPath(path))
	}
	for index := 0; index < len(node.Content); index += 2 {
		childPath := joinPointer(path, node.Content[index].Value)
		if err := validateOverlayCoverage(node.Content[index+1], childPath, planPaths); err != nil {
			return err
		}
	}
	return nil
}

func joinPointer(base, segment string) string {
	escaped := strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
	return base + "/" + escaped
}

func displayPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

// AllowedStrategies is shared with the versioned catalog and profile input
// validation so the UI cannot offer a strategy the composer cannot execute.
func AllowedStrategies(path string, kind yaml.Kind) []Strategy {
	switch kind {
	case yaml.ScalarNode, yaml.AliasNode:
		return []Strategy{StrategyReplace}
	case yaml.MappingNode:
		if _, named := namedMappingPaths[path]; named {
			return []Strategy{StrategyMergeByName, StrategyReplace}
		}
		return []Strategy{StrategyMerge, StrategyReplace}
	case yaml.SequenceNode:
		if path == "/rules" {
			return []Strategy{StrategyPrepend, StrategyAppendBeforeTerminal, StrategyReplace}
		}
		if _, named := namedSequencePaths[path]; named {
			return []Strategy{StrategyMergeByName, StrategyPrepend, StrategyAppend, StrategyReplace}
		}
		return []Strategy{StrategyPrepend, StrategyAppend, StrategyReplace}
	default:
		return nil
	}
}

func strategyAllowed(path string, node *yaml.Node, strategy Strategy) bool {
	for _, allowed := range AllowedStrategies(path, node.Kind) {
		if strategy == allowed {
			return true
		}
	}
	return false
}
