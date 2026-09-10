package application

import (
	"errors"
	"fmt"
	"jeemi/internal/chainproxy"

	"jeemi/internal/config/compose"
	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/config/geodataoverride"
	configinspect "jeemi/internal/config/inspect"
	"jeemi/internal/config/proxyoverride"
	configresources "jeemi/internal/config/resources"
	"jeemi/internal/config/ruleprovideroverride"
	"jeemi/internal/config/runtimeoverride"
	"jeemi/internal/config/schema"
	"jeemi/internal/geodata"
	"jeemi/internal/localscript"
	"jeemi/internal/profile"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscription"
	"jeemi/internal/subscriptionformat"
)

type composedSubscription struct {
	ChainProxy             chainproxy.Composition
	Normalization          subscriptionformat.Report
	Fallback               fallbackoverride.State
	Contents               []byte
	LocalConfigRevision    int
	LocalScriptRevision    int
	AvailableRuleProviders []configinspect.RuleProvider
}

const (
	projectionReady                  = "ready"
	projectionSourceUnavailable      = "source_unavailable"
	projectionNormalizationFailed    = "normalization_failed"
	projectionLocalConfigUnavailable = "local_config_unavailable"
	projectionLocalScriptUnavailable = "local_script_unavailable"
	projectionScriptExecutionFailed  = "script_execution_failed"
	projectionChainProxyFailed       = "chain_proxy_failed"
	projectionCompositionFailed      = "composition_failed"
	projectionInspectionFailed       = "inspection_failed"
)

func (s *Service) decorateSubscriptionState(state subscription.State) (subscription.State, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return subscription.State{}, err
	}
	selectedID := document.Subscriptions.SelectedID
	if !containsSubscription(state.Subscriptions, selectedID) {
		selectedID = ""
		if len(state.Subscriptions) > 0 {
			selectedID = state.Subscriptions[0].ID
		}
		if _, err := s.settingsStore.SetSelectedSubscription(selectedID); err != nil {
			return subscription.State{}, err
		}
	}

	state.SelectedSubscriptionID = selectedID
	state.Preferences = subscriptionPreferences(document)
	state.Projection = nil
	geoDataFingerprint, err := s.geoDataManager.DesiredFingerprint(document.GeoData)
	if err != nil {
		return subscription.State{}, err
	}
	for index := range state.Subscriptions {
		projection := s.subscriptionProjection(state.Subscriptions[index], document.Runtime, document.GeoData, geoDataFingerprint)
		state.Subscriptions[index].Composition = projection.Summary
		if state.Subscriptions[index].ID == selectedID {
			selected := projection
			state.Projection = &selected
		}
	}
	return state, nil
}

func containsSubscription(items []subscription.Summary, id string) bool {
	if id == "" {
		return len(items) == 0
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (s *Service) subscriptionProjection(summary subscription.Summary, runtimePreferences runtimeconfig.Preferences, geoDataPreferences geodata.Preferences, geoDataFingerprint string) subscription.Projection {
	projection := subscription.Projection{
		SubscriptionID:               summary.ID,
		RevisionID:                   summary.CurrentRevisionID,
		LocalConfigID:                summary.LocalConfigID,
		LocalScriptID:                summary.LocalScriptID,
		RuleProviderOverrideRevision: summary.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     summary.FallbackOverrideRevision,
		GeoDataFingerprint:           geoDataFingerprint,
		Status:                       projectionSourceUnavailable,
		CoreValidationStatus:         "not_run",
		Selectors:                    []subscription.Selector{},
		Proxies:                      []subscription.SelectorMember{},
		RuleProviders:                []subscription.RuleProvider{},
		Warnings:                     []string{},
	}
	composed, status, err := s.composeSubscription(summary, runtimePreferences, geoDataPreferences)
	if composed.Normalization.Format != "" {
		projection.Summary.Normalization = &composed.Normalization
		projection.NormalizationFingerprint = composed.Normalization.Fingerprint
	}
	if err != nil {
		var normalizationError *subscriptionformat.Error
		if errors.As(err, &normalizationError) {
			projection.Summary.NormalizationError = &subscriptionformat.Diagnostic{Code: normalizationError.Code, Line: normalizationError.Line, Field: normalizationError.Field}
		}
		projection.Status = status
		projection.Summary.Status = projection.Status
		return projection
	}
	projection.LocalConfigRevision = composed.LocalConfigRevision
	projection.LocalScriptRevision = composed.LocalScriptRevision
	projection.ChainProxy = composed.ChainProxy
	projection.ChainProxyFingerprint = composed.ChainProxy.Fingerprint
	inspected, err := configinspect.Configuration(composed.Contents)
	if err != nil {
		projection.Status = projectionInspectionFailed
		projection.Summary.Status = projection.Status
		return projection
	}
	projection.Fallback = composed.Fallback
	projection.Status = projectionReady
	projection.Warnings = append(projection.Warnings, inspected.Warnings...)
	for _, selector := range inspected.Selectors {
		converted := subscription.Selector{
			Name: selector.Name, Icon: selector.Icon, Hidden: selector.Hidden, Type: selector.Type, DefaultSelection: selector.DefaultSelection, ReferencedByRules: selector.ReferencedByRules,
			ProviderNames:           append([]string{}, selector.ProviderNames...),
			UnresolvedProviderNames: append([]string{}, selector.UnresolvedProviderNames...),
			Members:                 make([]subscription.SelectorMember, 0, len(selector.Members)),
		}
		for _, member := range selector.Members {
			converted.Members = append(converted.Members, subscription.SelectorMember{
				Name: member.Name, Type: member.Type, Source: member.Source, ProviderName: member.ProviderName,
			})
		}
		projection.Selectors = append(projection.Selectors, converted)
	}
	for _, proxy := range inspected.Proxies {
		projection.Proxies = append(projection.Proxies, subscription.SelectorMember{
			Name: proxy.Name, Type: proxy.Type, Source: proxy.Source, ProviderName: proxy.ProviderName,
		})
	}
	disabledProviders := stringSet(summary.DisabledRuleProviders)
	for _, provider := range composed.AvailableRuleProviders {
		_, disabled := disabledProviders[provider.Name]
		projection.RuleProviders = append(projection.RuleProviders, subscription.RuleProvider{
			Name: provider.Name, Type: provider.Type, Behavior: provider.Behavior, Format: provider.Format, Enabled: !disabled,
		})
	}
	projection.Summary = subscription.CompositionSummary{
		Normalization:      &composed.Normalization,
		Status:             projectionReady,
		ProxyCount:         inspected.ProxyCount,
		SelectorCount:      len(inspected.Selectors),
		ProxyProviderCount: inspected.ProxyProviderCount,
		RuleProviderCount:  len(inspected.RuleProviders),
	}
	return projection
}

func (s *Service) composeSubscription(summary subscription.Summary, runtimePreferences runtimeconfig.Preferences, geoDataPreferences geodata.Preferences) (composedSubscription, string, error) {
	return s.composeSubscriptionWithScript(summary, runtimePreferences, geoDataPreferences, nil)
}

// composeSubscriptionWithScript replaces the subscription's normal local
// handler when override is non-nil. It powers script testing and association
// validation without persisting an invalid relationship first.
func (s *Service) composeSubscriptionWithScript(summary subscription.Summary, runtimePreferences runtimeconfig.Preferences, geoDataPreferences geodata.Preferences, override *localscript.Script) (composedSubscription, string, error) {
	return s.composeSubscriptionCandidate(summary, runtimePreferences, geoDataPreferences, compositionCandidate{script: override})
}

// Candidate inputs replace only the requested layer in memory. Preview and
// validation must never publish a revision, association, or resolved snapshot.
type compositionCandidate struct {
	chains    *chainproxy.Library
	source    []byte
	local     *profile.LocalConfig
	resources *configresources.State
	script    *localscript.Script
}

func (s *Service) composeSubscriptionCandidate(summary subscription.Summary, runtimePreferences runtimeconfig.Preferences, geoDataPreferences geodata.Preferences, candidate compositionCandidate) (composedSubscription, string, error) {
	contents := candidate.source
	if contents == nil {
		text, err := s.subscriptions.Text(summary.ID)
		if err != nil {
			return composedSubscription{}, projectionSourceUnavailable, fmt.Errorf("read subscription configuration: %w", err)
		}
		contents = []byte(text.Contents)
	}
	normalized, err := subscriptionformat.Normalize(contents)
	if err != nil {
		return composedSubscription{}, projectionNormalizationFailed, err
	}
	override := candidate.script
	result := composedSubscription{Contents: normalized.Contents, Normalization: normalized.Report}
	if normalized.RequiredCore != "" {
		settings, loadErr := s.settingsStore.Load()
		if loadErr != nil {
			return result, projectionNormalizationFailed, loadErr
		}
		if settings.Mihomo.SelectedVersion != "" {
			if err := normalized.CheckCore(settings.Mihomo.SelectedVersion); err != nil {
				return result, projectionNormalizationFailed, err
			}
		}
	}
	if override == nil && summary.LocalConfigID != "" {
		local := candidate.local
		if local == nil {
			loaded, loadErr := s.localConfigs.Get(summary.LocalConfigID)
			if loadErr != nil {
				return composedSubscription{}, projectionLocalConfigUnavailable, fmt.Errorf("read associated local configuration: %w", loadErr)
			}
			local = &loaded
		}
		result.LocalConfigRevision = local.Revision
		result.Contents, err = configresources.PrepareSource(result.Contents, local.ResourcePlan)
		if err != nil {
			return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("prepare subscription routing for local reconstruction: %w", err)
		}
		composed, err := compose.Compose(result.Contents, []byte(local.OverlayYAML), mergePlan(local.Fields))
		if err != nil {
			return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("compose subscription and local configuration: %w", err)
		}
		result.Contents = []byte(composed.YAML)
		result.Contents, err = proxyoverride.Apply(result.Contents, proxyOverrides(local.Fields))
		if err != nil {
			return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("apply local proxy field overrides: %w", err)
		}
		resourceState := candidate.resources
		if resourceState == nil {
			loaded, resourceErr := s.configResources.State()
			if resourceErr != nil {
				return composedSubscription{}, projectionLocalConfigUnavailable, fmt.Errorf("read local configuration resources: %w", resourceErr)
			}
			resourceState = &loaded
		}
		result.Contents, err = configresources.Apply(result.Contents, *resourceState, local.ResourcePlan)
		if err != nil {
			return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("compose local strategy groups and rule sets: %w", err)
		}
	} else if override != nil || summary.LocalScriptID != "" {
		script := override
		if script == nil {
			loaded, loadErr := s.localScripts.Get(summary.LocalScriptID)
			if loadErr != nil {
				return composedSubscription{}, projectionLocalScriptUnavailable, fmt.Errorf("read associated local script: %w", loadErr)
			}
			script = &loaded
		}
		result.LocalScriptRevision = script.Revision
		result.Contents, err = localscript.Execute(script.Contents, result.Contents)
		if err != nil {
			return composedSubscription{}, projectionScriptExecutionFailed, fmt.Errorf("apply local script: %w", err)
		}
	}
	result.AvailableRuleProviders, err = configinspect.RuleProviders(result.Contents)
	if err != nil {
		return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("inspect configured rule providers: %w", err)
	}
	result.Contents, err = ruleprovideroverride.Apply(result.Contents, summary.DisabledRuleProviders)
	if err != nil {
		return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("apply rule provider switches: %w", err)
	}
	result.Contents, result.Fallback, err = fallbackoverride.Apply(result.Contents, summary.Fallback)
	if err != nil {
		return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("inject subscription fallback: %w", err)
	}
	if len(summary.ChainProxyGroupIDs) > 0 || summary.ChainProxyRevision > 0 {
		library := candidate.chains
		if library == nil {
			loaded, loadErr := s.chainProxies.Load()
			if loadErr != nil {
				return result, projectionChainProxyFailed, loadErr
			}
			library = &loaded
		}
		groups, chainErr := chainproxy.SelectedGroups(*library, summary.ChainProxyGroupIDs)
		if chainErr != nil {
			return result, projectionChainProxyFailed, chainErr
		}
		settings, loadErr := s.settingsStore.Load()
		if loadErr != nil {
			return result, projectionChainProxyFailed, loadErr
		}
		result.Contents, result.ChainProxy, chainErr = chainproxy.Apply(result.Contents, groups, summary.ChainProxyRevision, settings.Mihomo.SelectedVersion)
		if chainErr != nil {
			return result, projectionChainProxyFailed, chainErr
		}
	}
	result.Contents, err = runtimeoverride.Apply(result.Contents, runtimePreferences)
	if err != nil {
		return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("inject Jeemi runtime preferences: %w", err)
	}
	result.Contents, err = geodataoverride.Apply(result.Contents, geoDataPreferences)
	if err != nil {
		return composedSubscription{}, projectionCompositionFailed, fmt.Errorf("inject Jeemi GEO data preferences: %w", err)
	}
	return result, projectionReady, nil
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func mergePlan(fields []profile.LocalConfigField) []compose.Rule {
	plan := make([]compose.Rule, 0, len(fields))
	for _, field := range fields {
		definition, found := schema.FindField(field.Path)
		if !found || definition.Scope != schema.ScopeDirect {
			continue
		}
		plan = append(plan, compose.Rule{
			Path: field.Path, Strategy: field.Strategy, ConflictPolicy: field.ConflictPolicy,
		})
	}
	return plan
}

func proxyOverrides(fields []profile.LocalConfigField) []proxyoverride.Override {
	overrides := make([]proxyoverride.Override, 0)
	for _, field := range fields {
		definition, found := schema.FindField(field.Path)
		if !found || definition.Scope != schema.ScopeAllProxies {
			continue
		}
		overrides = append(overrides, proxyoverride.Override{
			Path:            field.Path,
			ValueYAML:       field.ValueYAML,
			TargetPath:      definition.TargetPath,
			ApplicableTypes: append([]string{}, definition.ApplicableTypes...),
		})
	}
	return overrides
}
