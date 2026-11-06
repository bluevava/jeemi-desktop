package application

import (
	"fmt"
	"jeemi/internal/config/presets"
	"jeemi/internal/config/resources"
	"jeemi/internal/profile"
)

// CreateLocalConfigPreset is an explicit user action. Listing presets and
// starting the client never seeds or rewrites the user's resource library.
func (s *Service) CreateLocalConfigPreset(kind string) (result profile.LocalConfig, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	p, err := presets.Load(kind)
	if err != nil {
		return result, err
	}
	state, err := s.configResources.PreparePreset(p.Plan, p.Groups, p.Rules)
	if err != nil {
		return result, err
	}
	configs, err := s.localConfigs.State()
	if err != nil {
		return result, err
	}
	names := map[string]bool{}
	for _, config := range configs.Configs {
		names[config.Name] = true
	}
	name := p.Name
	for n := 2; names[name]; n++ {
		name = fmt.Sprintf("%s (%d)", p.Name, n)
	}
	input := profile.SaveLocalConfigInput{Name: name, Description: p.Description, Fields: []profile.LocalConfigField{}, ResourcePlan: p.Plan}
	err = s.configResources.CommitImport(state, func(candidate resources.State, commit func() error) error {
		var saveErr error
		result, saveErr = s.localConfigs.SaveWithResourceCommit(input, candidate, commit)
		return saveErr
	})
	return result, err
}
