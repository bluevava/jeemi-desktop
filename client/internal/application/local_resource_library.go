package application

import (
	"fmt"
	"jeemi/internal/config/resources"
)

func (s *Service) resourceConfigReferences() ([]resources.ConfigReference, error) {
	state, err := s.localConfigs.State()
	if err != nil {
		return nil, err
	}
	result := []resources.ConfigReference{}
	for _, summary := range state.Configs {
		config, err := s.localConfigs.Get(summary.ID)
		if err != nil {
			return nil, err
		}
		ids := append([]string{}, config.ResourcePlan.StrategyGroupIDs...)
		seen := map[string]bool{}
		for _, id := range ids {
			seen[id] = true
		}
		for _, id := range []string{config.ResourcePlan.DefaultProxySelectorID, config.ResourcePlan.Match.SelectorID} {
			if id != "" && !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}
		result = append(result, resources.ConfigReference{ID: summary.ID, Name: summary.Name, StrategyGroupIDs: ids})
	}
	return result, nil
}

func (s *Service) resourceLibraryView(state resources.State) (resources.State, error) {
	refs, err := s.resourceConfigReferences()
	if err != nil {
		return resources.State{}, err
	}
	state.ConfigReferences = refs
	state.Platform = s.runtime.Platform.OS
	return state, nil
}

func (s *Service) DeleteLocalConfigResources(input resources.DeleteInput) (resources.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	refs, err := s.resourceConfigReferences()
	if err != nil {
		return resources.State{}, err
	}
	if input.Kind == "groups" {
		selected := map[string]bool{}
		for _, id := range input.IDs {
			selected[id] = true
		}
		for _, ref := range refs {
			for _, id := range ref.StrategyGroupIDs {
				if selected[id] {
					return resources.State{}, fmt.Errorf("strategy group is still referenced by a local configuration")
				}
			}
		}
	}
	state, err := s.configResources.DeleteValidated(input, nil)
	if err != nil {
		return resources.State{}, err
	}
	state.ConfigReferences = refs
	state.Platform = s.runtime.Platform.OS
	return state, nil
}
