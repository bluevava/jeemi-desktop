package application

import (
	"time"

	"jeemi/internal/config/resources"
	"jeemi/internal/configtransfer"
	"jeemi/internal/profile"
)

func (s *Service) validateLocalPackage(pending *pendingLocalPackage) ([]string, []string, error) {
	configs, subscriptions := []string{}, []string{}
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	if pending.kind == configtransfer.ScriptKind {
		candidate, err := s.localScripts.Preview(pending.script)
		if err != nil {
			return nil, nil, err
		}
		references, err := s.subscriptions.ReferencingLocalScript(pending.targetID)
		if err != nil {
			return nil, nil, err
		}
		for _, reference := range references {
			if _, err := s.validateLocalScriptCandidate(ctx, reference, candidate); err != nil {
				return nil, nil, candidateValidationError(reference, err)
			}
			subscriptions = append(subscriptions, reference.Name)
		}
		return configs, subscriptions, nil
	}
	preview, err := s.localConfigs.Preview(pending.config)
	if err != nil {
		return nil, nil, err
	}
	candidate := profile.LocalConfig{Fields: preview.Fields, OverlayYAML: preview.OverlayYAML, ResourcePlan: preview.ResourcePlan}
	state, err := s.localConfigs.State()
	if err != nil {
		return nil, nil, err
	}
	for _, summary := range state.Configs {
		local, err := s.localConfigs.Get(summary.ID)
		if err != nil {
			return nil, nil, err
		}
		changed := summary.ID == pending.targetID || pending.resources.ChangedGroupIDs[local.ResourcePlan.DefaultProxySelectorID] || pending.resources.ChangedGroupIDs[local.ResourcePlan.Match.SelectorID]
		for _, id := range local.ResourcePlan.StrategyGroupIDs {
			changed = changed || pending.resources.ChangedGroupIDs[id]
		}
		if !changed {
			continue
		}
		if summary.ID == pending.targetID {
			local = candidate
		}
		if _, err := resources.ValidatePlan(local.ResourcePlan, pending.resources.State); err != nil {
			return nil, nil, err
		}
		configs = append(configs, summary.Name)
		references, err := s.subscriptions.ReferencingLocalConfig(summary.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, reference := range references {
			if _, err := s.validateSubscriptionCandidate(ctx, reference, compositionCandidate{local: &local, resources: &pending.resources.State}); err != nil {
				return nil, nil, candidateValidationError(reference, err)
			}
			subscriptions = append(subscriptions, reference.Name)
		}
	}
	return configs, subscriptions, nil
}
