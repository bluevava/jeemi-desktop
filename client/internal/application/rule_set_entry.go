package application

import (
	"fmt"
	configresources "jeemi/internal/config/resources"
)

func (s *Service) PreviewRuleSetEntry(input configresources.RuleSetEntryInput) (result configresources.RuleSetEntryPreview, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	return s.configResources.PreviewRuleSetEntry(input)
}

func (s *Service) SaveRuleSetEntry(input configresources.RuleSetEntryInput) (result configresources.State, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.configResources.SaveRuleSetEntryValidated(input, func(candidate configresources.State) error {
		return s.validateResourceChange(candidate, "", input.RuleSetID)
	})
	if err != nil {
		return configresources.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalConfig, nil, reconcileAutomatic)
	return state, nil
}
