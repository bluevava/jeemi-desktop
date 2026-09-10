package application

import (
	"fmt"

	"jeemi/internal/config/runtimecontrol"
	mihomoruntime "jeemi/internal/mihomo/runtime"
)

const (
	runtimeConfigurationViewActive   = "active"
	runtimeConfigurationViewResolved = "resolved"
)

// RuntimeConfigurationText is a read-only, controller-session-free view of
// either the active mihomo generation or the latest resolved snapshot. It
// never exposes a managed file path, controller address, controller secret, or
// CORS policy.
type RuntimeConfigurationText struct {
	ChainProxyFingerprint        string `json:"chainProxyFingerprint"`
	Source                       string `json:"source"`
	Contents                     string `json:"contents"`
	GenerationID                 string `json:"generationId"`
	Fingerprint                  string `json:"fingerprint"`
	SubscriptionID               string `json:"subscriptionId"`
	SubscriptionRevision         string `json:"subscriptionRevision"`
	LocalConfigID                string `json:"localConfigId"`
	LocalConfigRevision          int    `json:"localConfigRevision"`
	LocalScriptID                string `json:"localScriptId"`
	LocalScriptRevision          int    `json:"localScriptRevision"`
	RuleProviderOverrideRevision int    `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int    `json:"fallbackOverrideRevision"`
	GeoDataFingerprint           string `json:"geoDataFingerprint"`
	CoreVersion                  string `json:"coreVersion"`
	CoreValidated                bool   `json:"coreValidated"`
	GeneratedAt                  string `json:"generatedAt"`
}

func (s *Service) RuntimeConfigurationText() (RuntimeConfigurationText, error) {
	live := s.runtimeManager.Status()
	if live.State == mihomoruntime.StateRunning {
		view, err := s.runtimeManager.ActiveConfiguration()
		if err != nil {
			return RuntimeConfigurationText{}, err
		}
		contents, err := runtimecontrol.FormatForDisplay(view.Contents)
		if err != nil {
			return RuntimeConfigurationText{}, err
		}
		configurationStatus := s.runtimeConfigurationStatus()
		return RuntimeConfigurationText{
			ChainProxyFingerprint: view.Source.ChainProxyFingerprint,
			Source:                runtimeConfigurationViewActive, Contents: string(contents),
			GenerationID: view.GenerationID, Fingerprint: configurationStatus.AppliedFingerprint,
			SubscriptionID: view.Source.SubscriptionID, SubscriptionRevision: view.Source.SubscriptionRevision,
			LocalConfigID: view.Source.LocalConfigID, LocalConfigRevision: view.Source.LocalConfigRevision,
			LocalScriptID: view.Source.LocalScriptID, LocalScriptRevision: view.Source.LocalScriptRevision,
			RuleProviderOverrideRevision: view.Source.RuleProviderOverrideRevision,
			FallbackOverrideRevision:     view.Source.FallbackOverrideRevision,
			GeoDataFingerprint:           view.Source.GeoDataFingerprint,
			CoreVersion:                  view.CoreVersion, CoreValidated: true, GeneratedAt: live.StartedAt,
		}, nil
	}

	configurationStatus := s.runtimeConfigurationStatus()
	subscriptionID := configurationStatus.SubscriptionID
	if subscriptionID == "" {
		document, err := s.settingsStore.Load()
		if err != nil {
			return RuntimeConfigurationText{}, err
		}
		subscriptionID = document.Subscriptions.SelectedID
	}
	if subscriptionID == "" {
		return RuntimeConfigurationText{}, fmt.Errorf("select a subscription before viewing the runtime configuration")
	}
	snapshot, err := s.resolvedConfigs.Current(subscriptionID)
	if err != nil {
		return RuntimeConfigurationText{}, err
	}
	contents, err := runtimecontrol.FormatForDisplay(snapshot.Configuration)
	if err != nil {
		return RuntimeConfigurationText{}, err
	}
	return RuntimeConfigurationText{
		ChainProxyFingerprint: snapshot.Manifest.ChainProxyFingerprint,
		Source:                runtimeConfigurationViewResolved, Contents: string(contents),
		Fingerprint:    snapshot.Manifest.Fingerprint,
		SubscriptionID: snapshot.Manifest.SubscriptionID, SubscriptionRevision: snapshot.Manifest.SubscriptionRevision,
		LocalConfigID: snapshot.Manifest.LocalConfigID, LocalConfigRevision: snapshot.Manifest.LocalConfigRevision,
		LocalScriptID: snapshot.Manifest.LocalScriptID, LocalScriptRevision: snapshot.Manifest.LocalScriptRevision,
		RuleProviderOverrideRevision: snapshot.Manifest.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     snapshot.Manifest.FallbackOverrideRevision,
		GeoDataFingerprint:           snapshot.Manifest.GeoDataFingerprint,
		CoreVersion:                  snapshot.Manifest.CoreVersion, CoreValidated: snapshot.Manifest.CoreValidated,
		GeneratedAt: snapshot.Manifest.GeneratedAt,
	}, nil
}
