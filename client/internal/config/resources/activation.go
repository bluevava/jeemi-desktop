package resources

// ApplyOptions selects the target platform without querying the host in the
// pure composer. Platform-specific resources remain portable in exports.
type ApplyOptions struct {
	Platform string
}

// activeResourceState is a projection. Shared definitions, payload comments and
// ordered references remain unchanged in storage and exported packages.
func activeResourceState(state State, platform string) State {
	rules := make(map[string]RuleSet, len(state.RuleSets))
	for _, rule := range state.RuleSets {
		rules[rule.ID] = rule
	}
	state.StrategyGroups = append([]StrategyGroup{}, state.StrategyGroups...)
	for i := range state.StrategyGroups {
		group := &state.StrategyGroups[i]
		refs := make([]RuleSetReference, 0, len(group.RuleSetReferences))
		for _, ref := range group.RuleSetReferences {
			rule, found := rules[ref.RuleSetID]
			if !group.Disabled && (!found || (!rule.Disabled && (platform == "" || rule.Platform == "" || rule.Platform == platform))) {
				refs = append(refs, ref)
			}
		}
		group.RuleSetReferences = refs
	}
	return state
}

func requiresPlatform(state State, plan Plan) bool {
	refs := make(map[string]bool)
	for _, group := range state.StrategyGroups {
		if group.Disabled || !contains(plan.StrategyGroupIDs, group.ID) || !plan.GroupEnabled(group.ID) {
			continue
		}
		for _, ref := range group.RuleSetReferences {
			refs[ref.RuleSetID] = true
		}
	}
	for _, rule := range state.RuleSets {
		if refs[rule.ID] && !rule.Disabled && rule.Platform != "" {
			return true
		}
	}
	return false
}
