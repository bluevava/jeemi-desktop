package resources

import (
	"jeemi/internal/config/compose"
	"jeemi/internal/config/document"
	"jeemi/internal/config/fallbackoverride"
)

// PrepareSource removes only subscription routing during a complete rebuild.
// The application calls it BEFORE injecting custom fields, so custom sub-rules
// and other YAML fields are not accidentally removed along with old routing.
func PrepareSource(contents []byte, input Plan) ([]byte, error) {
	plan, err := NormalizePlan(input)
	if err != nil {
		return nil, err
	}
	if plan.RulesDisabled || plan.RuleStrategy != compose.StrategyReplace {
		return contents, nil
	}
	parsed, err := document.Parse(contents)
	if err != nil {
		return nil, err
	}
	root := document.Root(parsed)
	// proxy-providers are node sources; they are intentionally preserved.
	for _, key := range []string{"rules", "proxy-groups", "rule-providers", "sub-rules"} {
		for index := 0; index < len(root.Content); index += 2 {
			if root.Content[index].Value == key {
				root.Content = append(root.Content[:index], root.Content[index+2:]...)
				break
			}
		}
	}
	return document.Encode(parsed)
}

func applyLocalMatch(contents []byte, plan Plan, groups map[string]StrategyGroup) ([]byte, error) {
	if plan.Match.Mode == "none" {
		return contents, nil
	}
	selection := fallbackoverride.Selection{Mode: plan.Match.Mode}
	if selection.Mode == "selector" {
		selection.Selector = strategyGroupDisplayName(groups[plan.Match.SelectorID])
	}
	result, _, err := fallbackoverride.Apply(contents, selection)
	return result, err
}

func hasProxyRules(groups []StrategyGroup) bool {
	for _, group := range groups {
		if group.Kind == StrategyGroupKindRule && group.Policy.Mode == PolicyProxy {
			return true
		}
	}
	return false
}

// Composition modes are derived from the single routing strategy, never saved
// as independent preferences that can disagree with a complete reconstruction.
func (plan Plan) selectorStrategy() compose.Strategy {
	if plan.RuleStrategy == compose.StrategyReplace {
		return compose.StrategyReplace
	}
	return compose.StrategyAppend
}

func (plan Plan) ruleProviderStrategy() compose.Strategy {
	if plan.RuleStrategy == compose.StrategyReplace {
		return compose.StrategyReplace
	}
	return compose.StrategyMergeByName
}
