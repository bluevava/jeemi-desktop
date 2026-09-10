package application

import (
	"context"
	"fmt"
	"image"
	"os"
	"sync"
	"time"

	"jeemi/internal/appupdate"
	"jeemi/internal/chainproxy"
	configresolved "jeemi/internal/config/resolved"
	configresources "jeemi/internal/config/resources"
	configschema "jeemi/internal/config/schema"
	"jeemi/internal/core"
	"jeemi/internal/geodata"
	"jeemi/internal/localscript"
	"jeemi/internal/mihomo/delaycache"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/platform"
	"jeemi/internal/platform/coreauth"
	platformfiles "jeemi/internal/platform/files"
	"jeemi/internal/platform/paths"
	"jeemi/internal/platform/processmemory"
	"jeemi/internal/platform/systemproxy"
	"jeemi/internal/profile"
	"jeemi/internal/settings"
	"jeemi/internal/subscription"
	"jeemi/internal/uidiagnostics"
)

const appName = "Jeemi"

// appVersion is replaced by the release build script through Go ldflags.
var appVersion = "0.1.0-dev"

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RuntimeStatus struct {
	CoreAuthorizationPending bool                       `json:"coreAuthorizationPending"`
	Platform                 platform.Info              `json:"platform"`
	Core                     core.Status                `json:"core"`
	ClientStartedAt          string                     `json:"clientStartedAt"`
	ProxyMode                string                     `json:"proxyMode"`
	SystemProxy              bool                       `json:"systemProxy"`
	TunEnabled               bool                       `json:"tunEnabled"`
	StatusMessageKey         string                     `json:"statusMessageKey"`
	Mihomo                   mihomoruntime.Status       `json:"mihomo"`
	Configuration            RuntimeConfigurationStatus `json:"configuration"`
	GeoData                  geodata.Health             `json:"geoData"`
	Memory                   processmemory.Snapshot     `json:"memory"`
}

type BootstrapState struct {
	App     AppInfo       `json:"app"`
	Runtime RuntimeStatus `json:"runtime"`
}

type Service struct {
	mu                   sync.RWMutex
	reconcileMu          sync.Mutex
	associationMu        sync.Mutex
	ctx                  context.Context
	runtime              RuntimeStatus
	versionManager       *core.VersionManager
	geoDataManager       *geodata.Manager
	settingsStore        *settings.Store
	localConfigs         *profile.LocalConfigStore
	localScripts         *localscript.Store
	configResources      *configresources.Store
	chainProxies         *chainproxy.Store
	chainFetcher         subscription.Fetcher
	chainOperationCancel context.CancelFunc
	subscriptions        *subscription.Manager
	runtimeManager       *mihomoruntime.Manager
	resolvedConfigs      *configresolved.Store
	delayCache           *delaycache.Store
	uiDiagnostics        *uidiagnostics.Store
	authorizeCore        func(context.Context, coreauth.Target) error
	coreOperationCancel  context.CancelFunc
	pendingRefresh       *pendingSubscriptionRefresh
	pendingPackage       *pendingLocalPackage
	shuttingDown         bool
	jeemiUpdater         *appupdate.Manager
}

func NewService(dataDirectory string) (*Service, error) {
	clientStartedAt := time.Now().UTC().Format(time.RFC3339Nano)
	settingsPath, err := paths.SettingsFileFromRoot(dataDirectory)
	if err != nil {
		return nil, err
	}
	settingsStore, err := settings.NewStore(settingsPath)
	if err != nil {
		return nil, err
	}
	manager, err := core.NewVersionManager(core.VersionManagerOptions{
		DataDirectory: dataDirectory,
		Settings:      settingsStore,
	})
	if err != nil {
		return nil, err
	}
	geoDataManager, err := geodata.NewManager(geodata.ManagerOptions{DataDirectory: dataDirectory})
	if err != nil {
		return nil, err
	}
	localConfigs, err := profile.NewLocalConfigStore(profile.LocalConfigStoreOptions{
		DataDirectory: dataDirectory,
	})
	if err != nil {
		return nil, err
	}
	localScripts, err := localscript.NewStore(localscript.StoreOptions{DataDirectory: dataDirectory})
	if err != nil {
		return nil, err
	}
	configResources, err := configresources.NewStore(configresources.StoreOptions{DataDirectory: dataDirectory})
	if err != nil {
		return nil, err
	}
	if _, err := configResources.State(); err != nil {
		return nil, err
	}
	var service *Service
	chainProxies, err := chainproxy.NewStore(dataDirectory)
	if err != nil {
		return nil, err
	}
	subscriptions, err := subscription.NewManager(subscription.ManagerOptions{
		DataDirectory: dataDirectory,
		ValidateImport: func(ctx context.Context, contents []byte) error {
			return service.validateImportedSubscription(ctx, contents)
		},
	})
	if err != nil {
		return nil, err
	}
	systemProxy, err := systemproxy.New(dataDirectory)
	if err != nil {
		return nil, err
	}
	runtimeManager, err := mihomoruntime.NewManager(mihomoruntime.Options{
		DataDirectory: dataDirectory,
		SystemProxy:   systemProxy,
	})
	if err != nil {
		return nil, err
	}
	resolvedConfigs, err := configresolved.NewStore(dataDirectory)
	if err != nil {
		return nil, err
	}
	delayCache, err := delaycache.NewStore(dataDirectory)
	if err != nil {
		return nil, err
	}
	service = &Service{
		runtime: RuntimeStatus{
			Platform:         platform.Current(),
			Core:             core.InitialStatus(),
			ClientStartedAt:  clientStartedAt,
			ProxyMode:        "off",
			StatusMessageKey: "runtime.status.frameworkReady",
			Configuration:    RuntimeConfigurationStatus{State: RuntimeConfigurationIdle},
		},
		versionManager:  manager,
		geoDataManager:  geoDataManager,
		settingsStore:   settingsStore,
		localConfigs:    localConfigs,
		localScripts:    localScripts,
		configResources: configResources,
		chainProxies:    chainProxies,
		chainFetcher:    subscription.NewHTTPFetcher(),
		subscriptions:   subscriptions,
		runtimeManager:  runtimeManager,
		resolvedConfigs: resolvedConfigs,
		delayCache:      delayCache,
		uiDiagnostics:   uidiagnostics.New(dataDirectory, appVersion),
		authorizeCore:   coreauth.Authorize,
		jeemiUpdater:    appupdate.NewManager(dataDirectory, appVersion),
	}
	service.refreshCoreStatus()
	return service, nil
}

func (s *Service) ConfigCatalog() configschema.Catalog {
	return configschema.Current()
}

func (s *Service) LocalConfigState() (profile.LocalConfigState, error) {
	state, err := s.localConfigs.State()
	if err != nil {
		return profile.LocalConfigState{}, err
	}
	resources, err := s.configResources.State()
	if err != nil {
		return profile.LocalConfigState{}, err
	}
	for index := range state.Configs {
		config, loadErr := s.localConfigs.Get(state.Configs[index].ID)
		if loadErr != nil {
			continue
		}
		count, countErr := configresources.RuleProviderCount(config.ResourcePlan, resources)
		if countErr != nil {
			state.Configs[index].StaticValidationStatus = "references_invalid"
			continue
		}
		state.Configs[index].RuleProviderCount = count
	}
	return state, nil
}

func (s *Service) LocalConfig(id string) (profile.LocalConfig, error) {
	config, err := s.localConfigs.Get(id)
	if err != nil {
		return profile.LocalConfig{}, err
	}
	resources, err := s.configResources.State()
	if err != nil {
		return profile.LocalConfig{}, err
	}
	count, err := configresources.RuleProviderCount(config.ResourcePlan, resources)
	if err != nil {
		config.StaticValidationStatus = "references_invalid"
		return config, nil
	}
	config.RuleProviderCount = count
	return config, nil
}

func (s *Service) PreviewLocalConfig(input profile.SaveLocalConfigInput) (profile.LocalConfigPreview, error) {
	resourceState, err := s.configResources.State()
	if err != nil {
		return profile.LocalConfigPreview{}, err
	}
	if _, err := configresources.ValidatePlan(input.ResourcePlan, resourceState); err != nil {
		return profile.LocalConfigPreview{}, err
	}
	return s.localConfigs.Preview(input)
}

func (s *Service) SaveLocalConfig(input profile.SaveLocalConfigInput) (result profile.LocalConfig, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	resourceState, err := s.configResources.State()
	if err != nil {
		return profile.LocalConfig{}, err
	}
	if _, err := configresources.ValidatePlan(input.ResourcePlan, resourceState); err != nil {
		return profile.LocalConfig{}, err
	}
	preview, err := s.localConfigs.Preview(input)
	if err != nil {
		return profile.LocalConfig{}, err
	}
	candidate := profile.LocalConfig{Fields: preview.Fields, OverlayYAML: preview.OverlayYAML, ResourcePlan: preview.ResourcePlan}
	references, err := s.subscriptions.ReferencingLocalConfig(input.ID)
	if input.ID == "" {
		references, err = nil, nil
	}
	if err != nil {
		return profile.LocalConfig{}, err
	}
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	for _, reference := range references {
		if _, err := s.validateSubscriptionCandidate(ctx, reference, compositionCandidate{local: &candidate, resources: &resourceState}); err != nil {
			return profile.LocalConfig{}, candidateValidationError(reference, err)
		}
	}
	saved, err := s.localConfigs.Save(input, resourceState)
	if err != nil {
		return profile.LocalConfig{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalConfig, nil, reconcileAutomatic)
	return saved, nil
}

func (s *Service) LocalConfigResources() (configresources.State, error) {
	return s.configResources.State()
}

func (s *Service) SaveStrategyGroup(input configresources.StrategyGroup) (result configresources.State, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.configResources.SaveStrategyGroupValidated(input, func(candidate configresources.State) error {
		return s.validateResourceChange(candidate, input.ID, "")
	})
	if err != nil {
		return configresources.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalConfig, nil, reconcileAutomatic)
	return state, nil
}

func (s *Service) DeleteStrategyGroup(id string) (configresources.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	referenced, err := s.localStrategyGroupReferenced(id)
	if err != nil {
		return configresources.State{}, err
	}
	if referenced {
		return configresources.State{}, fmt.Errorf("strategy group is still referenced by a local configuration")
	}
	return s.configResources.DeleteStrategyGroup(id)
}

func (s *Service) SaveRuleSet(input configresources.RuleSet) (result configresources.State, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s", safeConfigurationError(resultErr))
		}
	}()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.configResources.SaveRuleSetValidated(input, func(candidate configresources.State) error {
		return s.validateResourceChange(candidate, "", input.ID)
	})
	if err != nil {
		return configresources.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalConfig, nil, reconcileAutomatic)
	return state, nil
}

func (s *Service) DeleteRuleSet(id string) (configresources.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	return s.configResources.DeleteRuleSet(id)
}

func (s *Service) localStrategyGroupReferenced(id string) (bool, error) {
	state, err := s.localConfigs.State()
	if err != nil {
		return false, err
	}
	for _, summary := range state.Configs {
		config, err := s.localConfigs.Get(summary.ID)
		if err != nil {
			return false, err
		}
		for _, candidate := range config.ResourcePlan.StrategyGroupIDs {
			if candidate == id {
				return true, nil
			}
		}
		if config.ResourcePlan.DefaultProxySelectorID == id || config.ResourcePlan.Match.SelectorID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) DeleteLocalConfig(id string) (profile.LocalConfigState, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	references, err := s.subscriptions.ReferencingLocalConfig(id)
	if err != nil {
		return profile.LocalConfigState{}, err
	}
	if len(references) > 0 {
		return profile.LocalConfigState{}, fmt.Errorf("local configuration is still associated with %d subscription(s)", len(references))
	}
	return s.localConfigs.Delete(id)
}

func (s *Service) SubscriptionState() (subscription.State, error) {
	state, err := s.subscriptions.State()
	if err != nil {
		return subscription.State{}, err
	}
	return s.decorateSubscriptionState(state)
}

func (s *Service) Subscription(id string) (subscription.Detail, error) {
	return s.subscriptions.Get(id)
}

func (s *Service) ImportSubscriptionURL(input subscription.ImportURLInput) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	_, err := s.subscriptions.ImportURL(ctx, input)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) ImportSubscriptionFile(path string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	_, err := s.subscriptions.ImportFile(path)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) ImportSubscriptionQRCodeImage(path string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	_, err := s.subscriptions.ImportQRImage(ctx, path)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) CaptureSubscriptionQRCodeScreen(ctx context.Context) ([]image.Image, error) {
	return s.subscriptions.CaptureQRScreen(ctx)
}

func (s *Service) ImportSubscriptionQRCodeCapture(parent context.Context, images []image.Image) (subscription.State, error) {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if err := ctx.Err(); err != nil {
		return subscription.State{}, err
	}
	_, err := s.subscriptions.ImportQRScreenImages(ctx, images)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) RefreshSubscription(id string) (subscription.State, error) {
	result, err := s.CheckSubscriptionRefresh(id)
	if err != nil {
		return subscription.State{}, err
	}
	if result.Conflict != nil {
		return subscription.State{}, fmt.Errorf("subscription conflicts with its local handler; confirmation is required")
	}
	return *result.State, nil
}

func (s *Service) UpdateSubscription(input subscription.UpdateInput) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	_, err := s.subscriptions.UpdateValidated(ctx, input, func(summary subscription.Summary, source []byte) error {
		_, err := s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{source: source})
		return err
	})
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) DeleteSubscription(id string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	_, err := s.subscriptions.Delete(id)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.delayCache.Delete(id)
	_ = s.runtimeManager.ForgetProxySelections(id)
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) SubscriptionText(id string) (subscription.TextView, error) {
	return s.subscriptions.Text(id)
}

func (s *Service) SetSubscriptionLocalConfig(id, localConfigID string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	if localConfigID != "" {
		detail, err := s.subscriptions.Get(id)
		if err != nil {
			return subscription.State{}, err
		}
		if detail.LocalScriptID != "" {
			return subscription.State{}, fmt.Errorf("unlink the local script before associating a local configuration")
		}
		detail.LocalConfigID = localConfigID
		ctx, cancel := s.operationContext(30 * time.Second)
		_, validationErr := s.validateSubscriptionCandidate(ctx, detail.Summary, compositionCandidate{})
		cancel()
		if validationErr != nil {
			return subscription.State{}, candidateValidationError(detail.Summary, validationErr)
		}
	}
	_, err := s.subscriptions.SetLocalConfig(id, localConfigID)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) DetectSubscriptionIcon(sourceURL string) (subscription.IconDetectionResult, error) {
	ctx, cancel := s.operationContext(8 * time.Second)
	defer cancel()
	return s.subscriptions.DetectIcon(ctx, sourceURL)
}

func (s *Service) SetSubscriptionRuleProviderEnabled(id, providerName string, enabled bool) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.subscriptions.State()
	if err != nil {
		return subscription.State{}, err
	}
	var target subscription.Summary
	for _, item := range state.Subscriptions {
		if item.ID == id {
			target = item
			break
		}
	}
	if target.ID == "" {
		return subscription.State{}, fmt.Errorf("subscription does not exist")
	}
	document, err := s.settingsStore.Load()
	if err != nil {
		return subscription.State{}, err
	}
	composed, _, err := s.composeSubscription(target, document.Runtime, document.GeoData)
	if err != nil {
		return subscription.State{}, err
	}
	providerExists := false
	for _, provider := range composed.AvailableRuleProviders {
		if provider.Name == providerName {
			providerExists = true
			break
		}
	}
	if !providerExists {
		return subscription.State{}, fmt.Errorf("rule provider does not exist in the composed subscription")
	}
	_, err = s.subscriptions.SetRuleProviderEnabled(id, providerName, enabled)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) SelectSubscription(id string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.subscriptions.State()
	if err != nil {
		return subscription.State{}, err
	}
	found := false
	for _, item := range state.Subscriptions {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		return subscription.State{}, fmt.Errorf("subscription does not exist")
	}
	if _, err := s.settingsStore.SetSelectedSubscription(id); err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) SubscriptionPreferences() (subscription.Preferences, error) {
	document, err := s.settingsStore.Load()
	if err != nil {
		return subscription.Preferences{}, err
	}
	return subscriptionPreferences(document), nil
}

func (s *Service) SaveSubscriptionPreferences(input subscription.PreferencesInput) (subscription.Preferences, error) {
	document, err := s.settingsStore.SetSubscriptionPreferences(input.SelectorDensity, input.DelayTestConcurrency, input.ConnectionResetMode)
	if err != nil {
		return subscription.Preferences{}, err
	}
	return subscriptionPreferences(document), nil
}

func (s *Service) SaveSubscriptionSelectorDisplayPreferences(input subscription.SelectorDisplayPreferencesInput) (subscription.Preferences, error) {
	document, err := s.settingsStore.SetSubscriptionSelectorDisplayPreferences(input.SelectorSortMode, input.SelectorViewMode)
	if err != nil {
		return subscription.Preferences{}, err
	}
	return subscriptionPreferences(document), nil
}

func subscriptionPreferences(document settings.Document) subscription.Preferences {
	return subscription.Preferences{
		SelectorDensity:      document.Subscriptions.SelectorDensity,
		DelayTestConcurrency: document.Subscriptions.DelayTestConcurrency,
		ConnectionResetMode:  document.Subscriptions.ConnectionResetMode,
		SelectorSortMode:     document.Subscriptions.SelectorSortMode,
		SelectorViewMode:     document.Subscriptions.SelectorViewMode,
	}
}

func (s *Service) Startup(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.runtime.Platform = platform.Current()
	s.mu.Unlock()
	recoveryContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Known helper prerequisites stay idle until explicit authorization.
	if err := s.runtimeManager.RecoverSystemProxy(recoveryContext); err != nil && s.runtimeManager.Status().State == mihomoruntime.StateFailed {
		s.mu.Lock()
		s.runtime.StatusMessageKey = "runtime.status.systemProxyRecoveryFailed"
		s.mu.Unlock()
	}
	go func() {
		_ = s.reconcileSelected(reconcileTriggerStartup, nil, reconcileSnapshot)
	}()
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.CancelChainProxyRefresh()
	if s.jeemiUpdater != nil {
		s.jeemiUpdater.Cancel()
	}
	s.mu.Lock()
	s.shuttingDown = true
	cancel := s.coreOperationCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return s.runtimeManager.Shutdown(ctx)
}

func (s *Service) BootstrapState() BootstrapState {
	return BootstrapState{
		App:     AppInfo{Name: appName, Version: appVersion},
		Runtime: s.RuntimeStatus(),
	}
}

func (s *Service) RuntimeStatus() RuntimeStatus {
	s.refreshCoreStatus()
	s.mu.RLock()
	status := s.runtime
	if status.Configuration.LastError != nil {
		failure := *status.Configuration.LastError
		status.Configuration.LastError = &failure
	}
	s.mu.RUnlock()
	document, err := s.settingsStore.Load()
	if err != nil {
		status.GeoData = geodata.Health{Status: geodata.HealthInvalid, MissingKinds: []geodata.Kind{}, InvalidKinds: []geodata.Kind{}}
	} else if health, healthErr := s.geoDataManager.Health(document.GeoData); healthErr != nil {
		status.GeoData = geodata.Health{Status: geodata.HealthInvalid, MissingKinds: []geodata.Kind{}, InvalidKinds: []geodata.Kind{}}
	} else {
		status.GeoData = health
	}
	ctx, cancel := s.operationContext(1500 * time.Millisecond)
	defer cancel()
	status.Memory = captureRuntimeMemory(ctx, status.Mihomo.PID)
	return status
}

func (s *Service) MihomoVersionState() (core.VersionManagerState, error) {
	return s.versionManager.State()
}

func (s *Service) CheckMihomoUpdates() (core.VersionManagerState, error) {
	ctx, cancel := s.operationContext(30 * time.Second)
	defer cancel()
	return s.versionManager.Check(ctx)
}

func (s *Service) DownloadMihomoVersion(version string) (core.VersionManagerState, error) {
	s.setCoreStatus(core.Status{State: core.StateDownloading, Version: version})
	ctx, cancel := s.operationContext(15 * time.Minute)
	defer cancel()
	state, err := s.versionManager.Download(ctx, version)
	s.refreshCoreStatus()
	if err != nil {
		return core.VersionManagerState{}, err
	}
	return state, nil
}

func (s *Service) ImportMihomoCore(path string) (string, core.VersionManagerState, error) {
	ctx, cancel := s.operationContext(5 * time.Minute)
	defer cancel()
	version, state, err := s.versionManager.Import(ctx, path)
	s.refreshCoreStatus()
	if err != nil {
		return "", core.VersionManagerState{}, err
	}
	return version, state, nil
}

func (s *Service) CancelMihomoDownload() {
	s.versionManager.CancelDownload()
}

func (s *Service) SelectMihomoVersion(version string) (core.VersionManagerState, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	state, err := s.versionManager.Select(version)
	s.refreshCoreStatus()
	if err == nil {
		_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerCoreVersion, nil, reconcileAutomatic)
	}
	return state, err
}

func (s *Service) RemoveMihomoVersion(version string) (core.VersionManagerState, error) {
	state, err := s.versionManager.Remove(version)
	s.refreshCoreStatus()
	return state, err
}

func (s *Service) OpenMihomoDirectory() error {
	directory := s.versionManager.CoreDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create mihomo directory: %w", err)
	}
	return platformfiles.OpenDirectory(directory)
}

func (s *Service) operationContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *Service) refreshCoreStatus() {
	live := s.runtimeManager.Status()
	state, err := s.versionManager.State()
	if err != nil {
		s.mu.Lock()
		s.runtime.Core = core.Status{State: core.StateFailed}
		s.runtime.StatusMessageKey = "runtime.status.dataError"
		s.mu.Unlock()
		return
	}

	status := core.InitialStatus()
	messageKey := "runtime.status.frameworkReady"
	for _, installed := range state.InstalledVersions {
		if installed.Version == state.SelectedVersion {
			status = core.Status{State: core.StateReady, Version: installed.Version}
			messageKey = "runtime.status.coreReady"
			break
		}
	}
	if state.SelectedVersion != "" && status.State != core.StateReady {
		status = core.Status{State: core.StateFailed, Version: state.SelectedVersion}
		messageKey = "runtime.status.coreSelectionMissing"
	} else if state.SelectedVersion == "" && len(state.InstalledVersions) > 0 {
		messageKey = "runtime.status.coreInstalledNotSelected"
	}
	if state.DownloadingVersion != "" {
		status = core.Status{State: core.StateDownloading, Version: state.DownloadingVersion}
		messageKey = "runtime.status.coreDownloading"
	}

	if live.CoreVersion != "" || live.State != mihomoruntime.StateStopped {
		status.Version = live.CoreVersion
		status.ControllerReady = live.ControllerReady
		switch live.State {
		case mihomoruntime.StatePreparing, mihomoruntime.StateValidating, mihomoruntime.StateStarting,
			mihomoruntime.StateReloading, mihomoruntime.StateRecovering:
			status.State = core.StateStarting
			messageKey = "runtime.status.starting"
		case mihomoruntime.StateRunning:
			status.State = core.StateRunning
			messageKey = "runtime.status.running"
		case mihomoruntime.StateStopping:
			status.State = core.StateStopping
			messageKey = "runtime.status.stopping"
		case mihomoruntime.StateStopped:
			status.State = core.StateStopped
			messageKey = "runtime.status.stopped"
		case mihomoruntime.StateFailed:
			status.State = core.StateFailed
			if live.LastError != nil && live.LastError.Code == "startup_recovery_failed" {
				messageKey = "runtime.status.systemProxyRecoveryFailed"
			} else {
				messageKey = "runtime.status.failed"
			}
		}
	}

	s.mu.Lock()
	s.runtime.Core = status
	s.runtime.Mihomo = live
	s.runtime.ProxyMode = live.ProxyMode
	s.runtime.SystemProxy = live.SystemProxy
	s.runtime.TunEnabled = live.TUNEnabled
	s.runtime.StatusMessageKey = messageKey
	s.mu.Unlock()
}

func (s *Service) setCoreStatus(status core.Status) {
	s.mu.Lock()
	s.runtime.Core = status
	s.runtime.StatusMessageKey = "runtime.status.coreDownloading"
	s.mu.Unlock()
}
