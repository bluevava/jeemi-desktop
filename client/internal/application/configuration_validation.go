package application

import (
	"context"
	"fmt"
	"time"

	"jeemi/internal/config/fallbackoverride"
	configinspect "jeemi/internal/config/inspect"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/localscript"
	"jeemi/internal/subscription"
	"jeemi/internal/subscriptionformat"
)

// Converted imports must survive the same final composition as refreshes,
// before any raw revision is committed. Native import semantics stay unchanged.
func (s *Service) validateImportedSubscription(ctx context.Context, contents []byte) error {
	normalized, err := subscriptionformat.Normalize(contents)
	if err != nil || normalized.Report.Format == "mihomo" {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	_, err = s.validateSubscriptionCandidate(ctx, subscription.Summary{Fallback: fallbackoverride.Selection{Mode: fallbackoverride.ModeNone}}, compositionCandidate{source: contents})
	return err
}

func (s *Service) validateSubscriptionCandidate(ctx context.Context, summary subscription.Summary, candidate compositionCandidate) (localscript.TestResult, error) {
	if err := ctx.Err(); err != nil {
		return localscript.TestResult{}, err
	}
	document, err := s.settingsStore.Load()
	if err != nil {
		return localscript.TestResult{}, err
	}
	composed, _, err := s.composeSubscriptionCandidate(summary, document.Runtime, document.GeoData, candidate)
	if err != nil {
		return localscript.TestResult{}, err
	}
	safeConfiguration, err := runtimecontrol.StripSessionFields(composed.Contents)
	if err != nil {
		return localscript.TestResult{}, err
	}
	if err := configinspect.ValidateReferences(safeConfiguration); err != nil {
		return localscript.TestResult{}, fmt.Errorf("inspect final runtime configuration: %w", err)
	}
	if err := s.geoDataManager.ValidateDesiredRequirements(safeConfiguration, document.GeoData); err != nil {
		return localscript.TestResult{}, fmt.Errorf("validate final GEO requirements: %w", err)
	}
	coreValidated := false
	pendingGeo, err := s.geoDataManager.HasPending(document.GeoData)
	if err != nil {
		return localscript.TestResult{}, err
	}
	if installation, installErr := s.selectedMihomoInstallation(); installErr == nil && !pendingGeo {
		if err := s.geoDataManager.ValidateActiveRequirements(safeConfiguration, document.GeoData); err != nil {
			return localscript.TestResult{}, fmt.Errorf("validate active GEO requirements: %w", err)
		}
		if err := s.runtimeManager.ValidateConfiguration(ctx, installation.ExecutablePath, safeConfiguration); err != nil {
			return localscript.TestResult{}, fmt.Errorf("mihomo rejected final runtime configuration: %w", err)
		}
		coreValidated = true
	}
	if err := ctx.Err(); err != nil {
		return localscript.TestResult{}, err
	}
	return localscript.TestResult{
		SubscriptionID: summary.ID, SubscriptionName: summary.Name,
		Contents: string(safeConfiguration), CoreValidated: coreValidated,
	}, nil
}

// Validate every indirectly affected subscription, including inactive ones.
// The resource store passes its candidate while holding the commit lock, so
// this function must use that snapshot instead of re-entering the store.
func (s *Service) validateResourceChange(candidate configresources.State, groupID, ruleSetID string) error {
	if groupID == "" && ruleSetID == "" {
		return nil
	}
	affected := map[string]bool{}
	if groupID != "" {
		affected[groupID] = true
	}
	if ruleSetID != "" {
		for _, group := range candidate.StrategyGroups {
			for _, ref := range group.RuleSetReferences {
				if ref.RuleSetID == ruleSetID {
					affected[group.ID] = true
				}
			}
		}
	}
	configs, err := s.localConfigs.State()
	if err != nil {
		return err
	}
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	for _, item := range configs.Configs {
		local, err := s.localConfigs.Get(item.ID)
		if err != nil {
			return err
		}
		changed := affected[local.ResourcePlan.DefaultProxySelectorID] || affected[local.ResourcePlan.Match.SelectorID]
		for _, id := range local.ResourcePlan.StrategyGroupIDs {
			changed = changed || affected[id]
		}
		if !changed {
			continue
		}
		if _, err := configresources.ValidatePlan(local.ResourcePlan, candidate); err != nil {
			return err
		}
		references, err := s.subscriptions.ReferencingLocalConfig(local.ID)
		if err != nil {
			return err
		}
		for _, reference := range references {
			if _, err := s.validateSubscriptionCandidate(ctx, reference, compositionCandidate{local: &local, resources: &candidate}); err != nil {
				return candidateValidationError(reference, err)
			}
		}
	}
	return nil
}

func candidateValidationError(summary subscription.Summary, err error) error {
	return fmt.Errorf("subscription %q: %s", summary.Name, safeConfigurationError(err))
}
