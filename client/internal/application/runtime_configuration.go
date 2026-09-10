package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	configinspect "jeemi/internal/config/inspect"
	configresolved "jeemi/internal/config/resolved"
	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/core"
	"jeemi/internal/geodata"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscription"
)

type RuntimeConfigurationState string

const (
	RuntimeConfigurationIdle       RuntimeConfigurationState = "idle"
	RuntimeConfigurationBuilding   RuntimeConfigurationState = "building"
	RuntimeConfigurationValidating RuntimeConfigurationState = "validating"
	RuntimeConfigurationReady      RuntimeConfigurationState = "ready"
	RuntimeConfigurationApplying   RuntimeConfigurationState = "applying"
	RuntimeConfigurationApplied    RuntimeConfigurationState = "applied"
	RuntimeConfigurationFailed     RuntimeConfigurationState = "failed"
)

type RuntimeConfigurationStatus struct {
	ChainProxyFingerprint        string                    `json:"chainProxyFingerprint"`
	State                        RuntimeConfigurationState `json:"state"`
	Trigger                      string                    `json:"trigger"`
	DesiredFingerprint           string                    `json:"desiredFingerprint"`
	AppliedFingerprint           string                    `json:"appliedFingerprint"`
	SubscriptionID               string                    `json:"subscriptionId"`
	SubscriptionRevision         string                    `json:"subscriptionRevision"`
	LocalConfigID                string                    `json:"localConfigId"`
	LocalConfigRevision          int                       `json:"localConfigRevision"`
	LocalScriptID                string                    `json:"localScriptId"`
	LocalScriptRevision          int                       `json:"localScriptRevision"`
	RuleProviderOverrideRevision int                       `json:"ruleProviderOverrideRevision"`
	FallbackOverrideRevision     int                       `json:"fallbackOverrideRevision"`
	GeoDataFingerprint           string                    `json:"geoDataFingerprint"`
	CoreVersion                  string                    `json:"coreVersion"`
	CoreValidated                bool                      `json:"coreValidated"`
	UpdatedAt                    string                    `json:"updatedAt"`
	LastError                    *mihomoruntime.Failure    `json:"lastError"`
}

type reconciliationMode int

const (
	reconcileSnapshot reconciliationMode = iota
	reconcileAutomatic
	reconcileStart
	reconcileRestart
)

const (
	reconcileTriggerStartup            = "startup"
	reconcileTriggerRuntimePreferences = "runtime_preferences"
	reconcileTriggerSubscription       = "subscription"
	reconcileTriggerLocalConfig        = "local_config"
	reconcileTriggerLocalScript        = "local_script"
	reconcileTriggerCoreVersion        = "core_version"
	reconcileTriggerGeoData            = "geo_data"
	reconcileTriggerManualStart        = "manual_start"
	reconcileTriggerManualRestart      = "manual_restart"
)

type runtimeCandidate struct {
	request            mihomoruntime.StartRequest
	resolved           configresolved.Input
	fingerprint        string
	installation       *core.InstalledVersion
	geoDataPreferences geodata.Preferences
	geoDataFingerprint string
}

var errNoSelectedSubscription = errors.New("select a subscription before generating the runtime configuration")

func (s *Service) reconcileSelected(trigger string, previousPreferences *runtimeconfig.Preferences, mode reconciliationMode) error {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	return s.reconcileSelectedWithTimeoutLocked(trigger, previousPreferences, mode)
}

func (s *Service) reconcileSelectedWithTimeoutLocked(trigger string, previousPreferences *runtimeconfig.Preferences, mode reconciliationMode) error {
	ctx, cancel := s.operationContext(100 * time.Second)
	defer cancel()
	return s.reconcileSelectedLocked(ctx, trigger, previousPreferences, mode)
}

func (s *Service) reconcileSelectedLocked(ctx context.Context, trigger string, previousPreferences *runtimeconfig.Preferences, mode reconciliationMode) error {
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.State = RuntimeConfigurationBuilding
		status.Trigger = trigger
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		status.LastError = nil
	})

	if err := s.normalizeSubscriptionFallbacks(); err != nil {
		return s.failRuntimeConfiguration(trigger, "compose", "fallback_update_failed", err, nil)
	}
	candidate, err := s.buildRuntimeCandidate()
	if err != nil {
		if errors.Is(err, errNoSelectedSubscription) && mode != reconcileStart && mode != reconcileRestart {
			live := s.runtimeManager.Status()
			if mode == reconcileSnapshot || (live.State == mihomoruntime.StateStopped && !live.DesiredRunning) {
				s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
					status.State = RuntimeConfigurationIdle
					status.DesiredFingerprint = ""
					status.AppliedFingerprint = ""
					status.SubscriptionID = ""
					status.SubscriptionRevision = ""
					status.LocalConfigID = ""
					status.LocalConfigRevision = 0
					status.LocalScriptID = ""
					status.LocalScriptRevision = 0
					status.ChainProxyFingerprint = ""
					status.RuleProviderOverrideRevision = 0
					status.FallbackOverrideRevision = 0
					status.GeoDataFingerprint = ""
					status.CoreVersion = ""
					status.CoreValidated = false
					status.LastError = nil
				})
				return nil
			}
		}
		if errors.Is(err, errNoSelectedSubscription) {
			return s.failRuntimeConfiguration(trigger, "compose", "subscription_required", err, nil)
		}
		return s.failRuntimeConfiguration(trigger, "compose", "configuration_build_failed", err, nil)
	}
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.DesiredFingerprint = candidate.fingerprint
		status.SubscriptionID = candidate.resolved.SubscriptionID
		status.SubscriptionRevision = candidate.resolved.SubscriptionRevision
		status.LocalConfigID = candidate.resolved.LocalConfigID
		status.LocalConfigRevision = candidate.resolved.LocalConfigRevision
		status.LocalScriptID = candidate.resolved.LocalScriptID
		status.LocalScriptRevision = candidate.resolved.LocalScriptRevision
		status.ChainProxyFingerprint = candidate.resolved.ChainProxyFingerprint
		status.RuleProviderOverrideRevision = candidate.resolved.RuleProviderOverrideRevision
		status.FallbackOverrideRevision = candidate.resolved.FallbackOverrideRevision
		status.GeoDataFingerprint = candidate.geoDataFingerprint
		status.CoreVersion = candidate.resolved.CoreVersion
		status.CoreValidated = false
	})

	live := s.runtimeManager.Status()
	geoPending, err := s.geoDataManager.HasPending(candidate.geoDataPreferences)
	if err != nil {
		return s.failRuntimeConfiguration(trigger, "geodata", "geodata_state_failed", err, nil)
	}
	fastMode := !geoPending && candidate.installation != nil && live.State == mihomoruntime.StateRunning &&
		previousPreferences != nil && onlyOutboundModeChanged(*previousPreferences, candidate.resolved.Preferences) &&
		sameRuntimeSource(live.Source, candidate.request.Source) && live.CoreVersion == candidate.installation.Version

	var geoActivation *geodata.Activation
	inactiveRuntime := live.State == mihomoruntime.StateStopped || (live.State == mihomoruntime.StateFailed && live.PID == 0)
	if geoPending {
		if err := s.geoDataManager.ValidateDesiredRequirements(candidate.resolved.Configuration, candidate.geoDataPreferences); err != nil {
			return s.failRuntimeConfiguration(trigger, "geodata", "geodata_required_asset_missing", err, nil)
		}
	}
	if geoPending && inactiveRuntime && candidate.installation != nil {
		geoActivation, err = s.geoDataManager.PrepareActivation(candidate.geoDataPreferences)
		if err != nil {
			return s.failRuntimeConfiguration(trigger, "geodata", "geodata_activation_failed", err, nil)
		}
	}
	rollbackGeo := func() {
		if geoActivation != nil {
			_ = geoActivation.Rollback()
			geoActivation = nil
		}
	}

	if !geoPending {
		err = s.geoDataManager.ValidateActiveRequirements(candidate.resolved.Configuration, candidate.geoDataPreferences)
	}
	if err != nil {
		rollbackGeo()
		return s.failRuntimeConfiguration(trigger, "geodata", "geodata_required_asset_missing", err, nil)
	}

	if candidate.installation != nil && !fastMode {
		s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
			status.State = RuntimeConfigurationValidating
		})
		// A running core may still have an older memory-mapped database. Each
		// pending revision was already validated in an isolated home, so defer
		// exact full-config validation until the controlled restart has swapped
		// the files. Stopped runtimes validate against the newly activated files.
		if !(geoPending && geoActivation == nil && live.State == mihomoruntime.StateRunning) {
			if err := s.runtimeManager.ValidateConfiguration(ctx, candidate.installation.ExecutablePath, candidate.resolved.Configuration); err != nil {
				rollbackGeo()
				return s.failRuntimeConfiguration(trigger, "validate", "configuration_validation_failed", err, nil)
			}
			candidate.resolved.CoreValidated = true
		}
	} else if fastMode {
		// The source and every structural preference are identical to the
		// active, already validated generation. Only the validated mode enum
		// changes and can use mihomo's high-frequency control API.
		candidate.resolved.CoreValidated = true
	}

	snapshot, err := s.resolvedConfigs.Save(candidate.resolved)
	if err != nil {
		rollbackGeo()
		return s.failRuntimeConfiguration(trigger, "persist", "configuration_persist_failed", err, nil)
	}
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.DesiredFingerprint = snapshot.Manifest.Fingerprint
		status.CoreValidated = snapshot.Manifest.CoreValidated
		status.UpdatedAt = snapshot.Manifest.GeneratedAt
	})

	activationRequested := mode == reconcileStart || mode == reconcileRestart ||
		(mode == reconcileAutomatic && (live.State == mihomoruntime.StateRunning || live.DesiredRunning))
	if geoPending && geoActivation == nil && live.State == mihomoruntime.StateRunning &&
		mode != reconcileStart && mode != reconcileRestart {
		// GEO files are intentionally not replaced beneath a running core.
		// Keep the desired snapshot visible and wait for an explicit restart.
		s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
			status.State = RuntimeConfigurationReady
			status.LastError = nil
		})
		return nil
	}
	if !activationRequested {
		if geoActivation != nil {
			if err := geoActivation.Commit(); err != nil {
				geoActivation = nil
				return s.failRuntimeConfiguration(trigger, "geodata", "geodata_activation_failed", err, nil)
			}
			geoActivation = nil
		}
		s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
			status.State = RuntimeConfigurationReady
			status.LastError = nil
		})
		return nil
	}
	if !fastMode {
		if err := s.runtimeManager.CheckPrerequisites(ctx, candidate.request, mode == reconcileRestart || geoPending); err != nil {
			rollbackGeo()
			return s.failRuntimeConfiguration(trigger, "prepare", "platform_unavailable", err, nil)
		}
	}
	if candidate.installation == nil {
		rollbackGeo()
		return s.failRuntimeConfiguration(trigger, "prepare", "core_unavailable", fmt.Errorf("select an installed mihomo version before starting"), nil)
	}

	current := s.runtimeConfigurationStatus()
	if !geoPending && mode == reconcileAutomatic && current.AppliedFingerprint == snapshot.Manifest.Fingerprint &&
		live.State == mihomoruntime.StateRunning && sameRuntimeSource(live.Source, candidate.request.Source) &&
		live.CoreVersion == candidate.installation.Version {
		s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
			status.State = RuntimeConfigurationApplied
			status.LastError = nil
		})
		return nil
	}

	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.State = RuntimeConfigurationApplying
	})
	if fastMode && mode == reconcileAutomatic {
		modeContext, cancel := context.WithTimeout(ctx, 8*time.Second)
		err = s.runtimeManager.SetMode(modeContext, candidate.resolved.Preferences.OutboundMode)
		cancel()
	} else {
		var previousRequest *mihomoruntime.StartRequest
		if geoPending && geoActivation == nil {
			if live.State == mihomoruntime.StateRunning {
				request, requestErr := s.runtimeManager.ActiveStartRequest()
				if requestErr != nil {
					return s.failRuntimeConfiguration(trigger, "geodata", "geodata_rollback_point_failed", requestErr, nil)
				}
				previousRequest = &request
			}
			if live.State != mihomoruntime.StateStopped {
				stopContext, cancel := context.WithTimeout(ctx, 15*time.Second)
				_, err = s.runtimeManager.Stop(stopContext)
				cancel()
			}
			if err == nil {
				geoActivation, err = s.geoDataManager.PrepareActivation(candidate.geoDataPreferences)
			}
		} else if mode == reconcileRestart && live.State != mihomoruntime.StateStopped {
			stopContext, cancel := context.WithTimeout(ctx, 15*time.Second)
			_, err = s.runtimeManager.Stop(stopContext)
			cancel()
		}
		if err == nil {
			_, err = s.runtimeManager.Start(ctx, candidate.request)
		}
		if err != nil && geoActivation != nil {
			rollbackErr := geoActivation.Rollback()
			geoActivation = nil
			if previousRequest != nil {
				restoreContext, cancel := context.WithTimeout(context.Background(), 100*time.Second)
				_, restoreErr := s.runtimeManager.Start(restoreContext, *previousRequest)
				cancel()
				if restoreErr != nil {
					err = fmt.Errorf("%v; restore previous mihomo runtime: %w", err, restoreErr)
				}
			}
			if rollbackErr != nil {
				err = fmt.Errorf("%v; restore previous GEO data: %w", err, rollbackErr)
			}
		} else if err != nil && previousRequest != nil {
			restoreContext, cancel := context.WithTimeout(context.Background(), 100*time.Second)
			_, restoreErr := s.runtimeManager.Start(restoreContext, *previousRequest)
			cancel()
			if restoreErr != nil {
				err = fmt.Errorf("%v; restore previous mihomo runtime: %w", err, restoreErr)
			}
		}
	}
	if err != nil {
		rollbackGeo()
		failure := s.runtimeManager.Status().LastError
		return s.failRuntimeConfiguration(trigger, "apply", "configuration_apply_failed", err, failure)
	}
	if geoActivation != nil {
		if err := geoActivation.Commit(); err != nil {
			geoActivation = nil
			return s.failRuntimeConfiguration(trigger, "geodata", "geodata_activation_failed", err, nil)
		}
		geoActivation = nil
	}
	if !candidate.resolved.CoreValidated {
		candidate.resolved.CoreValidated = true
		if validatedSnapshot, saveErr := s.resolvedConfigs.Save(candidate.resolved); saveErr == nil {
			snapshot = validatedSnapshot
		} else {
			return s.failRuntimeConfiguration(trigger, "persist", "configuration_persist_failed", saveErr, nil)
		}
	}
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.State = RuntimeConfigurationApplied
		status.AppliedFingerprint = snapshot.Manifest.Fingerprint
		status.LastError = nil
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	})
	return nil
}

func (s *Service) buildRuntimeCandidate() (runtimeCandidate, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return runtimeCandidate{}, err
	}
	state, err := s.subscriptions.State()
	if err != nil {
		return runtimeCandidate{}, err
	}
	selectedID := document.Subscriptions.SelectedID
	if !containsSubscription(state.Subscriptions, selectedID) {
		selectedID = ""
		if len(state.Subscriptions) > 0 {
			selectedID = state.Subscriptions[0].ID
		}
		if _, err := s.settingsStore.SetSelectedSubscription(selectedID); err != nil {
			return runtimeCandidate{}, err
		}
	}
	var selected subscription.Summary
	for _, item := range state.Subscriptions {
		if item.ID == selectedID {
			selected = item
			break
		}
	}
	if selected.ID == "" {
		return runtimeCandidate{}, errNoSelectedSubscription
	}
	composed, _, err := s.composeSubscription(selected, document.Runtime, document.GeoData)
	if err != nil {
		return runtimeCandidate{}, err
	}
	safeConfiguration, err := runtimecontrol.StripSessionFields(composed.Contents)
	if err != nil {
		return runtimeCandidate{}, err
	}
	if _, err := configinspect.Configuration(safeConfiguration); err != nil {
		return runtimeCandidate{}, fmt.Errorf("inspect resolved runtime configuration: %w", err)
	}

	geoDataFingerprint, err := s.geoDataManager.DesiredFingerprint(document.GeoData)
	if err != nil {
		return runtimeCandidate{}, err
	}
	input := configresolved.Input{
		ChainProxyFingerprint:    composed.ChainProxy.Fingerprint,
		NormalizationFingerprint: composed.Normalization.Fingerprint,
		SubscriptionID:           selected.ID, SubscriptionRevision: selected.CurrentRevisionID,
		LocalConfigID: selected.LocalConfigID, LocalConfigRevision: composed.LocalConfigRevision,
		LocalScriptID: selected.LocalScriptID, LocalScriptRevision: composed.LocalScriptRevision,
		RuleProviderOverrideRevision: selected.RuleProviderOverrideRevision,
		FallbackOverrideRevision:     selected.FallbackOverrideRevision,
		Preferences:                  document.Runtime, GeoDataPreferences: document.GeoData,
		GeoDataFingerprint: geoDataFingerprint, Configuration: safeConfiguration,
	}
	candidate := runtimeCandidate{
		resolved: input, geoDataPreferences: document.GeoData, geoDataFingerprint: geoDataFingerprint,
	}
	if installed, installErr := s.selectedMihomoInstallation(); installErr == nil {
		candidate.installation = &installed
		candidate.resolved.CoreVersion = installed.Version
		candidate.request = mihomoruntime.StartRequest{
			ExecutablePath: installed.ExecutablePath, CoreVersion: installed.Version,
			Configuration: safeConfiguration, Preferences: document.Runtime,
			Source: mihomoruntime.Source{
				ChainProxyFingerprint:    composed.ChainProxy.Fingerprint,
				NormalizationFingerprint: composed.Normalization.Fingerprint,
				SubscriptionID:           selected.ID, SubscriptionRevision: selected.CurrentRevisionID,
				LocalConfigID: selected.LocalConfigID, LocalConfigRevision: composed.LocalConfigRevision,
				LocalScriptID: selected.LocalScriptID, LocalScriptRevision: composed.LocalScriptRevision,
				RuleProviderOverrideRevision: selected.RuleProviderOverrideRevision,
				FallbackOverrideRevision:     selected.FallbackOverrideRevision,
				GeoDataFingerprint:           geoDataFingerprint,
			},
		}
	}
	fingerprint, _, err := configresolved.Fingerprint(candidate.resolved)
	if err != nil {
		return runtimeCandidate{}, err
	}
	candidate.fingerprint = fingerprint
	return candidate, nil
}

func onlyOutboundModeChanged(previous, next runtimeconfig.Preferences) bool {
	if previous.OutboundMode == next.OutboundMode {
		return false
	}
	previous.OutboundMode = next.OutboundMode
	return runtimeconfig.Equal(previous, next)
}

func sameRuntimeSource(left, right mihomoruntime.Source) bool {
	return left.ChainProxyFingerprint == right.ChainProxyFingerprint && left.NormalizationFingerprint == right.NormalizationFingerprint &&
		left.SubscriptionID == right.SubscriptionID &&
		left.SubscriptionRevision == right.SubscriptionRevision &&
		left.LocalConfigID == right.LocalConfigID &&
		left.LocalConfigRevision == right.LocalConfigRevision &&
		left.LocalScriptID == right.LocalScriptID &&
		left.LocalScriptRevision == right.LocalScriptRevision &&
		left.RuleProviderOverrideRevision == right.RuleProviderOverrideRevision &&
		left.FallbackOverrideRevision == right.FallbackOverrideRevision &&
		left.GeoDataFingerprint == right.GeoDataFingerprint
}

func (s *Service) failRuntimeConfiguration(trigger, phase, code string, err error, failure *mihomoruntime.Failure) error {
	if failure == nil {
		failure = &mihomoruntime.Failure{Code: requirements.Code(err, code), Phase: phase, Message: safeConfigurationError(err)}
	} else {
		copy := *failure
		failure = &copy
	}
	s.updateConfigurationStatus(func(status *RuntimeConfigurationStatus) {
		status.State = RuntimeConfigurationFailed
		status.Trigger = trigger
		status.LastError = failure
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	})
	return err
}

func (s *Service) updateConfigurationStatus(update func(*RuntimeConfigurationStatus)) {
	s.mu.Lock()
	update(&s.runtime.Configuration)
	s.mu.Unlock()
}

func (s *Service) runtimeConfigurationStatus() RuntimeConfigurationStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.runtime.Configuration
	if status.LastError != nil {
		failure := *status.LastError
		status.LastError = &failure
	}
	return status
}
