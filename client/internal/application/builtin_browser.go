package application

import (
	"jeemi/internal/config/resources"
	"jeemi/internal/platform/builtinbrowser"
)

func (s *Service) localResourceOptions(state resources.State, plan resources.Plan) (resources.ApplyOptions, error) {
	if !resources.UsesBuiltInBrowser(state, plan) {
		return resources.ApplyOptions{}, nil
	}
	rules, err := builtinbrowser.Rules(s.runtime.Platform.OS)
	return resources.ApplyOptions{BuiltInBrowserRules: rules}, err
}
