package resources

import (
	"fmt"
	"sort"
)

// ImportCandidate is an in-memory replacement; preparing it never writes files.
type ImportCandidate struct {
	State               State
	Plan                Plan
	OverwrittenGroups   []string
	OverwrittenRuleSets []string
	AddedGroups         []string
	AddedRuleSets       []string
	ChangedGroupIDs     map[string]bool
}

// ExportGraph includes the default selector even when it is not in the rule order.
func ExportGraph(plan Plan, state State) ([]StrategyGroup, []RuleSet, error) {
	if _, err := ValidatePlan(plan, state); err != nil {
		return nil, nil, err
	}
	wanted := map[string]bool{}
	for _, id := range plan.StrategyGroupIDs {
		wanted[id] = true
	}
	if plan.DefaultProxySelectorID != "" {
		wanted[plan.DefaultProxySelectorID] = true
	}
	if plan.Match.SelectorID != "" {
		wanted[plan.Match.SelectorID] = true
	}
	groups, rules := []StrategyGroup{}, []RuleSet{}
	ruleIDs := map[string]bool{}
	for _, group := range state.StrategyGroups {
		if !wanted[group.ID] {
			continue
		}
		groups = append(groups, group)
		for _, ref := range group.RuleSetReferences {
			ruleIDs[ref.RuleSetID] = true
		}
	}
	for _, rule := range state.RuleSets {
		if ruleIDs[rule.ID] {
			rules = append(rules, rule)
		}
	}
	if len(groups) != len(wanted) || len(rules) != len(ruleIDs) {
		return nil, nil, fmt.Errorf("package references unavailable resources")
	}
	return groups, rules, nil
}

func (s *Store) PrepareImport(plan Plan, groups []StrategyGroup, rules []RuleSet) (ImportCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return ImportCandidate{}, err
	}
	result := ImportCandidate{State: state, Plan: plan, OverwrittenGroups: []string{}, OverwrittenRuleSets: []string{}, AddedGroups: []string{}, AddedRuleSets: []string{}, ChangedGroupIDs: map[string]bool{}}
	// Normalize detached copies. Neither package IDs nor timestamps become new identities.
	groups = append([]StrategyGroup{}, groups...)
	rules = append([]RuleSet{}, rules...)
	now := s.now().UTC()
	for i := range rules {
		rules[i].CreatedAt = ""
		rules[i], err = normalizeRuleSet(rules[i], now)
		if err != nil {
			return ImportCandidate{}, err
		}
		if rules[i].Behavior == "classical" && rules[i].Format == "mrs" {
			return ImportCandidate{}, fmt.Errorf("classical rule sets cannot use mrs format")
		}
	}
	for i := range groups {
		groups[i].CreatedAt = ""
		groups[i].RuleSetReferences = append([]RuleSetReference{}, groups[i].RuleSetReferences...)
		groups[i], err = normalizeStrategyGroup(groups[i], now)
		if err != nil {
			return ImportCandidate{}, err
		}
	}
	groupNames := map[string]bool{}
	for _, group := range groups {
		if groupNames[group.Name] {
			return ImportCandidate{}, fmt.Errorf("package contains duplicate strategy group names")
		}
		groupNames[group.Name] = true
		matches := 0
		for _, existing := range state.StrategyGroups {
			if existing.Name == group.Name {
				matches++
			}
		}
		if matches > 1 {
			return ImportCandidate{}, fmt.Errorf("multiple local strategy groups share the name %q; rename them before importing", group.Name)
		}
	}
	if err := validateState(groups, rules); err != nil {
		return ImportCandidate{}, err
	}
	exportedGroups, exportedRules, err := ExportGraph(plan, State{StrategyGroups: groups, RuleSets: rules})
	if err != nil {
		return ImportCandidate{}, err
	}
	if len(exportedGroups) != len(groups) || len(exportedRules) != len(rules) {
		return ImportCandidate{}, fmt.Errorf("package contains unrelated resources")
	}
	ruleIDs, groupIDs := map[string]string{}, map[string]string{}
	usedIDs := map[string]bool{}
	for _, rule := range state.RuleSets {
		usedIDs[rule.ID] = true
	}
	for _, group := range state.StrategyGroups {
		usedIDs[group.ID] = true
	}
	newID := func() (string, error) {
		id, err := s.newID()
		if err != nil || !resourceIDPattern.MatchString(id) || usedIDs[id] {
			return "", fmt.Errorf("create imported resource id")
		}
		usedIDs[id] = true
		return id, nil
	}
	for _, rule := range rules {
		originalID, index := rule.ID, -1
		for i, existing := range state.RuleSets {
			if existing.Name == rule.Name {
				index = i
				rule.ID, rule.CreatedAt = existing.ID, existing.CreatedAt
				break
			}
		}
		if index < 0 {
			rule.ID, err = newID()
			if err != nil {
				return ImportCandidate{}, err
			}
			result.State.RuleSets = append(result.State.RuleSets, rule)
			result.AddedRuleSets = append(result.AddedRuleSets, rule.Name)
		} else {
			result.State.RuleSets[index] = rule
			result.OverwrittenRuleSets = append(result.OverwrittenRuleSets, rule.Name)
		}
		ruleIDs[originalID] = rule.ID
	}
	for _, group := range groups {
		originalID, index := group.ID, -1
		for i, existing := range state.StrategyGroups {
			if existing.Name == group.Name {
				index = i
				group.ID, group.CreatedAt = existing.ID, existing.CreatedAt
				break
			}
		}
		for i := range group.RuleSetReferences {
			group.RuleSetReferences[i].RuleSetID = ruleIDs[group.RuleSetReferences[i].RuleSetID]
		}
		if index < 0 {
			group.ID, err = newID()
			if err != nil {
				return ImportCandidate{}, err
			}
			result.State.StrategyGroups = append(result.State.StrategyGroups, group)
			result.AddedGroups = append(result.AddedGroups, group.Name)
		} else {
			// Full-package replacement may change kind; every referencing plan is
			// checked by the application before this candidate can be committed.
			result.State.StrategyGroups[index] = group
			result.OverwrittenGroups = append(result.OverwrittenGroups, group.Name)
		}
		groupIDs[originalID] = group.ID
		result.ChangedGroupIDs[group.ID] = true
	}
	result.Plan.StrategyGroupIDs = append([]string{}, plan.StrategyGroupIDs...)
	for i, id := range result.Plan.StrategyGroupIDs {
		result.Plan.StrategyGroupIDs[i] = groupIDs[id]
	}
	result.Plan.DisabledStrategyGroupIDs = append([]string{}, plan.DisabledStrategyGroupIDs...)
	for i, id := range result.Plan.DisabledStrategyGroupIDs {
		result.Plan.DisabledStrategyGroupIDs[i] = groupIDs[id]
	}
	result.Plan.DefaultProxySelectorID = groupIDs[plan.DefaultProxySelectorID]
	result.Plan.Match.SelectorID = groupIDs[plan.Match.SelectorID]
	if err := validateState(result.State.StrategyGroups, result.State.RuleSets); err != nil {
		return ImportCandidate{}, err
	}
	if _, err := ValidatePlan(result.Plan, result.State); err != nil {
		return ImportCandidate{}, err
	}
	for _, names := range [][]string{result.OverwrittenGroups, result.OverwrittenRuleSets, result.AddedGroups, result.AddedRuleSets} {
		sort.Strings(names)
	}
	return result, nil
}

// CommitImport holds the library lock while the config store stages its files.
// The config store invokes commitLibrary last and restores its previous directory
// if that atomic library replacement fails. Readers cannot observe a partial save.
func (s *Store) CommitImport(candidate State, saveConfig func(State, func() error) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if candidate.Revision != current.Revision {
		return fmt.Errorf("resource library changed; select the package again")
	}
	if err := validateState(candidate.StrategyGroups, candidate.RuleSets); err != nil {
		return err
	}
	return saveConfig(candidate, func() error { _, err := s.saveUnlocked(candidate); return err })
}
