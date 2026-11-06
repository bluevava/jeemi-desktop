package resources

import "fmt"

// ApplyOptions supplies platform rules without giving the pure composer
// filesystem, process or registry access.
type ApplyOptions struct {
	BuiltInBrowserRules []string
}

func UsesBuiltInBrowser(state State, plan Plan) bool {
	if plan.RulesDisabled {
		return false
	}
	refs := make(map[string]bool)
	for _, id := range plan.StrategyGroupIDs {
		if !plan.GroupEnabled(id) {
			continue
		}
		for _, group := range state.StrategyGroups {
			if group.ID == id {
				for _, ref := range group.RuleSetReferences {
					refs[ref.RuleSetID] = true
				}
			}
		}
	}
	for _, rule := range state.RuleSets {
		if refs[rule.ID] && rule.DefaultBrowser {
			return true
		}
	}
	return false
}

func resolveBuiltInBrowser(state State, plan Plan, options []ApplyOptions) (State, error) {
	if !UsesBuiltInBrowser(state, plan) {
		return state, nil
	}
	if len(options) == 0 || len(options[0].BuiltInBrowserRules) == 0 {
		return State{}, fmt.Errorf("system built-in browser rules are unavailable; configure browser process rules manually")
	}
	if err := validateRuleSetPayloadRules("classical", options[0].BuiltInBrowserRules); err != nil {
		return State{}, err
	}
	state.RuleSets = append([]RuleSet{}, state.RuleSets...)
	for i := range state.RuleSets {
		if state.RuleSets[i].DefaultBrowser {
			state.RuleSets[i].Payload = append(append([]string{}, state.RuleSets[i].Payload...), options[0].BuiltInBrowserRules...)
		}
	}
	return state, nil
}
