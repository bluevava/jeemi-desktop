package resources

import "fmt"

// DeleteValidated commits the complete selection once. A stale revision,
// missing resource or live reference rejects the whole candidate.
func (s *Store) DeleteValidated(input DeleteInput, validate func(State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (input.Kind != "groups" && input.Kind != "rules") || len(input.IDs) == 0 || len(input.IDs) > 10000 {
		return State{}, fmt.Errorf("invalid resource deletion selection")
	}
	state, err := s.loadUnlocked()
	if err != nil {
		return State{}, err
	}
	if state.Revision != input.ExpectedRevision {
		return State{}, fmt.Errorf("resource library changed; refresh before deleting")
	}
	ids := map[string]bool{}
	for _, id := range input.IDs {
		if !resourceIDPattern.MatchString(id) || ids[id] {
			return State{}, fmt.Errorf("invalid resource deletion selection")
		}
		ids[id] = true
	}
	found := 0
	if input.Kind == "groups" {
		next := make([]StrategyGroup, 0, len(state.StrategyGroups))
		for _, item := range state.StrategyGroups {
			if ids[item.ID] {
				found++
				continue
			}
			next = append(next, item)
		}
		state.StrategyGroups = next
	} else {
		for _, group := range state.StrategyGroups {
			for _, ref := range group.RuleSetReferences {
				if ids[ref.RuleSetID] {
					return State{}, fmt.Errorf("rule set is still referenced by a strategy group")
				}
			}
		}
		next := make([]RuleSet, 0, len(state.RuleSets))
		for _, item := range state.RuleSets {
			if ids[item.ID] {
				found++
				continue
			}
			next = append(next, item)
		}
		state.RuleSets = next
	}
	if found != len(ids) {
		return State{}, fmt.Errorf("selected resource does not exist")
	}
	if err := validateState(state.StrategyGroups, state.RuleSets); err != nil {
		return State{}, err
	}
	if validate != nil {
		if err := validate(state); err != nil {
			return State{}, err
		}
	}
	return s.saveUnlocked(state)
}
