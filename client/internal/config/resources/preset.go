package resources

import "fmt"

// PreparePreset only adds missing resources. Stable preset IDs let the two
// configurations share their business groups without overwriting local edits.
func (s *Store) PreparePreset(plan Plan, groups []StrategyGroup, rules []RuleSet) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	names, ruleNames := map[string]bool{}, map[string]bool{}
	groupIDs, ruleIDs := map[string]bool{}, map[string]bool{}
	for _, g := range state.StrategyGroups {
		groupIDs[g.ID] = true
		names[strategyGroupDisplayName(g)] = true
		if g.AutoSelect {
			names[strategyGroupDisplayName(g)+" Auto"] = true
		}
	}
	for _, r := range state.RuleSets {
		ruleIDs[r.ID] = true
		ruleNames[r.Name] = true
	}
	unique := func(name string, used map[string]bool, emitted func(string) []string) string {
		base := name
		for i := 2; ; i++ {
			collision := false
			for _, candidate := range emitted(name) {
				collision = collision || used[candidate]
			}
			if !collision {
				break
			}
			name = fmt.Sprintf("%s (%d)", base, i)
		}
		for _, candidate := range emitted(name) {
			used[candidate] = true
		}
		return name
	}
	now := s.now().UTC()
	for _, r := range rules {
		if ruleIDs[r.ID] {
			continue
		}
		r.Name = unique(r.Name, ruleNames, func(name string) []string { return []string{name} })
		r, err = normalizeRuleSet(r, now)
		if err != nil {
			return State{}, err
		}
		state.RuleSets = append(state.RuleSets, r)
	}
	for _, g := range groups {
		if groupIDs[g.ID] {
			continue
		}
		g.Name = unique(g.Name, names, func(name string) []string {
			candidate := g
			candidate.Name = name
			display := strategyGroupDisplayName(candidate)
			result := []string{display}
			if g.AutoSelect {
				result = append(result, display+" Auto")
			}
			return result
		})
		g, err = normalizeStrategyGroup(g, now)
		if err != nil {
			return State{}, err
		}
		state.StrategyGroups = append(state.StrategyGroups, g)
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err != nil {
		return State{}, err
	}
	if _, err := ValidatePlan(plan, state); err != nil {
		return State{}, err
	}
	return state, nil
}
