package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"jeemi/internal/platform/paths"
	"jeemi/internal/platform/requirements"
	"jeemi/internal/runtimeconfig"
)

type SystemProxyController interface {
	Apply(context.Context, runtimeconfig.Preferences) error
	Restore(context.Context) error
	Recover(context.Context) error
}

type Options struct {
	DataDirectory string
	Driver        Driver
	SystemProxy   SystemProxyController
	HTTPClient    *http.Client
	Now           func() time.Time
	GOOS          string
	RestartDelays []time.Duration
	ObserveTUN    func(context.Context, string) error
}

type Manager struct {
	operationMu  sync.Mutex
	mu           sync.RWMutex
	dnsMu        sync.Mutex
	dnsID        string
	dnsCancel    context.CancelFunc
	dnsCancelled map[string]bool

	store                  *generationStore
	selections             selectionStore
	forgottenSubscriptions map[string]bool
	driver                 Driver
	systemProxy            SystemProxyController
	httpClient             *http.Client
	now                    func() time.Time
	goos                   string
	restartDelays          []time.Duration
	observeTUN             func(context.Context, string) error
	status                 Status
	process                Process
	recoveryBlocked        Process
	active                 *Generation
	client                 *controllerClient
	shutdown               chan struct{}
	shutdownOnce           sync.Once

	startupFailure              error
	startupAuthorizationPending bool
}

func NewManager(options Options) (*Manager, error) {
	driver := options.Driver
	if driver == nil {
		driver = newCommandDriver()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	store, err := newGenerationStore(options.DataDirectory, driver, now)
	if err != nil {
		return nil, err
	}
	selectionRoot, err := paths.ProxySelectionsDirectoryFromRoot(options.DataDirectory)
	if err != nil {
		return nil, err
	}
	goos := options.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	restartDelays := append([]time.Duration{}, options.RestartDelays...)
	if len(restartDelays) == 0 {
		restartDelays = []time.Duration{time.Second, 3 * time.Second, 10 * time.Second}
	}
	observer := options.ObserveTUN
	if observer == nil {
		observer = waitForTUNInterface
	}
	proxy := options.SystemProxy
	if proxy == nil {
		proxy = unavailableSystemProxy{}
	}
	return &Manager{
		store: store, driver: driver, systemProxy: proxy, httpClient: options.HTTPClient,
		selections:             selectionStore{root: selectionRoot},
		forgottenSubscriptions: map[string]bool{},
		now:                    now, goos: goos, restartDelays: restartDelays, observeTUN: observer,
		status: Status{State: StateStopped, ProxyMode: "off"}, shutdown: make(chan struct{}),
	}, nil
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

// ValidateConfiguration checks a long-lived resolved snapshot with the
// selected official core without creating a controller session or changing
// process/system-network state. Start and reload still validate the exact
// session generation again after injecting fresh control fields.
func (m *Manager) ValidateConfiguration(ctx context.Context, executablePath string, configuration []byte) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	select {
	case <-m.shutdown:
		return fmt.Errorf("mihomo runtime is shutting down")
	default:
	}
	return m.store.ValidateResolved(ctx, executablePath, configuration)
}

func (m *Manager) RecoverSystemProxy(ctx context.Context) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.recoverSystemProxyLocked(ctx)
}

func (m *Manager) recoverSystemProxyLocked(ctx context.Context) error {
	cleanupErr := m.store.EraseAllSessionFiles()
	restoreErr := m.systemProxy.Recover(ctx)
	// An old or missing helper is handled by the next explicit authorization
	// action. Keep recovery pending without reporting an idle core as failed.
	// A simultaneous session cleanup failure must still remain visible.
	authorizationPending := cleanupErr == nil && startupAuthorizationRequired(restoreErr)
	if cleanupErr != nil && restoreErr != nil {
		cleanupErr = fmt.Errorf("erase stale mihomo session: %v; restore system proxy: %w", cleanupErr, restoreErr)
	} else if restoreErr != nil {
		cleanupErr = restoreErr
	}
	m.mu.Lock()
	m.startupFailure = cleanupErr
	m.startupAuthorizationPending = authorizationPending
	if authorizationPending {
		m.status.DesiredRunning = false
		m.status.State = StateStopped
		m.status.LastError = nil
	} else if cleanupErr != nil {
		m.status.DesiredRunning = false
		m.status.State = StateFailed
		m.status.LastError = &Failure{Code: "startup_recovery_failed", Phase: "startup", Message: safeErrorMessage(cleanupErr)}
	} else if m.status.LastError != nil && m.status.LastError.Code == "startup_recovery_failed" {
		m.status.State = StateStopped
		m.status.LastError = nil
	}
	m.mu.Unlock()
	return cleanupErr
}

func (m *Manager) Start(ctx context.Context, request StartRequest) (Status, error) {
	m.cancelActiveDNSQuery()
	if err := validateStartRequest(request); err != nil {
		m.mu.RLock()
		status := cloneStatus(m.status)
		running := m.status.State == StateRunning
		m.mu.RUnlock()
		if running {
			return status, err
		}
		return m.fail("invalid_request", "prepare", err)
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	select {
	case <-m.shutdown:
		return m.fail("runtime_closed", "prepare", fmt.Errorf("mihomo runtime is shutting down"))
	default:
	}
	m.mu.RLock()
	startupFailure := m.startupFailure
	authorizationPending := m.startupAuthorizationPending
	m.mu.RUnlock()
	if startupFailure != nil {
		if authorizationPending {
			return m.Status(), startupFailure
		}
		return m.fail("startup_recovery_failed", "prepare", startupFailure)
	}
	// Check before stopping or reloading a healthy generation. A Linux desktop
	// or privilege failure must not take the previous working proxy offline.
	if err := m.preflight(ctx, request, false); err != nil {
		code := requirements.Code(err, "platform_unavailable")
		if m.Status().State == StateRunning {
			return m.failRunning(code, "prepare", err)
		}
		return m.fail(code, "prepare", err)
	}
	m.mu.Lock()
	m.status.DesiredRunning = true
	m.status.LastError = nil
	m.mu.Unlock()

	m.mu.RLock()
	running := m.process != nil && m.status.State == StateRunning
	active := m.active
	m.mu.RUnlock()
	if running && active != nil && sameExecutable(m.goos, active.ExecutablePath, request.ExecutablePath) {
		return m.reloadLocked(ctx, request)
	}
	if running {
		if err := m.stopCurrentLocked(ctx, false); err != nil {
			return m.fail("stop_failed", "restart", err)
		}
	} else if active != nil && active.Session.Secret != "" {
		// A manual start can overtake the bounded recovery delay after an
		// unexpected exit. Retire that dead controller session before creating
		// the replacement so its secret cannot remain on disk indefinitely.
		if err := m.store.EraseSessionFiles(active.Session); err != nil {
			m.mu.Lock()
			m.status.DesiredRunning = false
			m.mu.Unlock()
			return m.fail("session_cleanup_failed", "prepare", err)
		}
		m.mu.Lock()
		if m.active == active {
			m.active.Session = ControllerSession{}
		}
		m.mu.Unlock()
	}

	m.mu.Lock()
	m.status.CoreVersion = request.CoreVersion
	m.status.SubscriptionID = request.Source.SubscriptionID
	m.status.Source = request.Source
	m.status.OutboundMode = request.Preferences.OutboundMode
	m.mu.Unlock()
	m.setState(StateValidating)
	generation, err := m.store.Prepare(ctx, request, nil)
	if err != nil {
		m.mu.Lock()
		m.status.DesiredRunning = false
		m.mu.Unlock()
		return m.fail("validation_failed", "validate", err)
	}
	if err := m.launchLocked(ctx, generation, 0); err != nil {
		if cleanupErr := m.store.EraseGenerationSessionFiles(generation); cleanupErr != nil {
			err = fmt.Errorf("%v; erase failed runtime session: %w", err, cleanupErr)
		}
		m.mu.Lock()
		m.status.DesiredRunning = false
		m.mu.Unlock()
		return m.fail(requirements.Code(err, "start_failed"), "start", err)
	}
	return m.Status(), nil
}

func (m *Manager) Stop(ctx context.Context) (Status, error) {
	m.cancelActiveDNSQuery()
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.mu.Lock()
	m.status.DesiredRunning = false
	m.mu.Unlock()
	if err := m.stopCurrentLocked(ctx, true); err != nil {
		return m.fail("stop_failed", "stop", err)
	}
	return m.Status(), nil
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.shutdownOnce.Do(func() { close(m.shutdown) })
	_, err := m.Stop(ctx)
	return err
}

func (m *Manager) SetMode(ctx context.Context, mode string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	client, err := m.runningClient()
	if err != nil {
		return err
	}
	if err := client.SetMode(ctx, mode); err != nil {
		return err
	}
	m.mu.Lock()
	if m.active != nil {
		m.active.Preferences.OutboundMode = mode
	}
	m.status.OutboundMode = mode
	m.status.LastError = nil
	m.mu.Unlock()
	return nil
}

func (m *Manager) SelectProxy(ctx context.Context, group, proxy string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	client, err := m.runningClient()
	if err != nil {
		return err
	}
	if err := client.SelectProxy(ctx, group, proxy); err != nil {
		return err
	}
	return m.rememberProxySelection(ctx, client, group, proxy)
}

func (m *Manager) UpdateRuleProvider(ctx context.Context, name string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	client, err := m.runningClient()
	if err != nil {
		return err
	}
	return client.UpdateRuleProvider(ctx, name)
}

func (m *Manager) UpdateProxyProvider(ctx context.Context, name string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	client, err := m.runningClient()
	if err != nil {
		return err
	}
	return client.UpdateProxyProvider(ctx, name)
}

func (m *Manager) reloadLocked(ctx context.Context, request StartRequest) (Status, error) {
	m.mu.RLock()
	active := m.active
	client := m.client
	m.mu.RUnlock()
	if active == nil || client == nil {
		return m.fail("session_unavailable", "reload", fmt.Errorf("mihomo controller session is unavailable"))
	}
	if err := m.captureProxySelections(ctx, client, *active); err != nil {
		return m.failRunning("proxy_selection_save_failed", "reload", err)
	}
	m.setState(StateValidating)
	generation, err := m.store.Prepare(ctx, request, &active.Session)
	if err != nil {
		m.setState(StateRunning)
		return m.failRunning("validation_failed", "validate", err)
	}
	m.setState(StateReloading)
	if active.Preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy {
		if err := m.systemProxy.Restore(ctx); err != nil {
			if cleanupErr := m.store.EraseGenerationSessionFiles(generation); cleanupErr != nil {
				err = fmt.Errorf("%v; erase failed runtime session: %w", err, cleanupErr)
			}
			m.setState(StateRunning)
			return m.failRunning("proxy_restore_failed", "reload", err)
		}
	}
	if network, ok := m.driver.(managedNetwork); ok && active.Preferences.ProxyMode == runtimeconfig.ProxyModeTUN {
		if err := network.DeactivateNetwork(ctx); err != nil {
			m.setState(StateRunning)
			return m.failRunning("proxy_restore_failed", "reload", err)
		}
	}
	reloadCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	err = m.reloadConfiguration(reloadCtx, client, generation.ConfigPath)
	cancel()
	if err == nil {
		healthCtx, healthCancel := context.WithTimeout(ctx, 12*time.Second)
		err = client.WaitHealthy(healthCtx)
		healthCancel()
	}
	if err == nil {
		err = m.activateNetwork(ctx, generation, client, true)
	}
	if err == nil {
		if activationErr := m.store.Activate(generation); activationErr != nil {
			err = fmt.Errorf("persist active generation: %w", activationErr)
		}
	}
	if err != nil {
		if generation.Preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy {
			_ = m.systemProxy.Restore(context.Background())
		}
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 35*time.Second)
		rollbackErr := m.reloadConfiguration(rollbackCtx, client, active.ConfigPath)
		rollbackCancel()
		if rollbackErr == nil {
			modeCtx, modeCancel := context.WithTimeout(context.Background(), 5*time.Second)
			rollbackErr = client.SetMode(modeCtx, active.Preferences.OutboundMode)
			modeCancel()
		}
		if rollbackErr == nil {
			rollbackErr = m.activateNetwork(context.Background(), *active, client, true)
		}
		if cleanupErr := m.store.EraseGenerationSessionFiles(generation); cleanupErr != nil {
			err = fmt.Errorf("%v; erase failed runtime session: %w", err, cleanupErr)
		}
		if rollbackErr != nil {
			m.mu.Lock()
			m.status.DesiredRunning = false
			m.mu.Unlock()
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 8*time.Second)
			stopErr := m.stopCurrentLocked(stopCtx, false)
			stopCancel()
			if stopErr != nil {
				return m.fail("reload_rollback_failed", "rollback", fmt.Errorf("reload, rollback, and runtime stop failed: %w", stopErr))
			}
			return m.fail("reload_rollback_failed", "rollback", fmt.Errorf("reload failed and rollback failed"))
		}
		m.setState(StateRunning)
		return m.failRunning("reload_failed", "reload", err)
	}
	m.mu.Lock()
	m.active = &generation
	m.status.State = StateRunning
	m.status.GenerationID = generation.ID
	m.status.SubscriptionID = generation.Source.SubscriptionID
	m.status.Source = generation.Source
	m.status.OutboundMode = generation.Preferences.OutboundMode
	m.status.ProxyMode = generation.Preferences.ProxyMode
	m.status.SystemProxy = generation.Preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy
	m.status.TUNEnabled = generation.Preferences.ProxyMode == runtimeconfig.ProxyModeTUN
	m.status.TUNDevice = m.tunDevice()
	m.status.LastError = nil
	m.mu.Unlock()
	return m.Status(), nil
}

func (m *Manager) launchLocked(ctx context.Context, generation Generation, restartAttempt int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Every actual process launch, including rollback and automatic recovery,
	// rechecks the new process's privileges. This never requests authentication.
	if err := m.preflight(ctx, StartRequest{ExecutablePath: generation.ExecutablePath, Preferences: generation.Preferences}, true); err != nil {
		return err
	}
	m.setState(StateStarting)
	process, err := m.driver.Start(generation.ExecutablePath, m.store.root, generation.BootstrapPath)
	if err != nil {
		return err
	}
	client := newControllerClient(m.httpClient, generation.Session)
	m.mu.Lock()
	m.process = process
	m.recoveryBlocked = nil
	m.client = nil
	m.status.PID = process.PID()
	m.status.CoreVersion = generation.CoreVersion
	m.status.RestartAttempt = restartAttempt
	m.mu.Unlock()

	healthCtx, healthCancel := context.WithTimeout(ctx, 25*time.Second)
	err = client.WaitHealthy(healthCtx)
	healthCancel()
	if err == nil && processExited(process) {
		err = fmt.Errorf("mihomo exited during startup")
	}
	if err == nil {
		err = m.activateNetwork(ctx, generation, client, false)
	}
	// A fast API mode switch updates the in-memory active generation so an
	// unexpected process restart restores the latest user choice. Apply it
	// after TUN activation because loading the final configuration can reset
	// the mode contained in the bootstrap configuration.
	if err == nil {
		modeCtx, modeCancel := context.WithTimeout(ctx, 5*time.Second)
		err = client.SetMode(modeCtx, generation.Preferences.OutboundMode)
		modeCancel()
	}
	if err == nil && processExited(process) {
		err = fmt.Errorf("mihomo exited while activating system networking")
	}
	if err == nil {
		err = m.store.Activate(generation)
	}
	if err != nil {
		m.deactivateNetwork(client, generation.Preferences)
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = process.Stop(stopCtx)
		cancel()
		m.mu.Lock()
		if m.process == process {
			m.process = nil
		}
		m.status.PID = 0
		m.mu.Unlock()
		return err
	}

	startedAt := m.now().UTC().Format(time.RFC3339Nano)
	m.mu.Lock()
	m.active = &generation
	m.client = client
	m.status = Status{
		State: StateRunning, DesiredRunning: true, CoreVersion: generation.CoreVersion,
		PID: process.PID(), GenerationID: generation.ID, SubscriptionID: generation.Source.SubscriptionID,
		Source:          generation.Source,
		ControllerReady: true, ControllerSession: cloneSession(&generation.Session),
		OutboundMode: generation.Preferences.OutboundMode,
		ProxyMode:    generation.Preferences.ProxyMode,
		SystemProxy:  generation.Preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy,
		TUNEnabled:   generation.Preferences.ProxyMode == runtimeconfig.ProxyModeTUN,
		TUNDevice:    m.tunDevice(), StartedAt: startedAt,
		RestartAttempt: restartAttempt,
	}
	m.mu.Unlock()
	m.watch(process)
	return nil
}

func (m *Manager) activateNetwork(ctx context.Context, generation Generation, client *controllerClient, configurationLoaded bool) error {
	if generation.Preferences.ProxyMode == runtimeconfig.ProxyModeTUN {
		var err error
		if !configurationLoaded {
			reloadCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			err = m.reloadConfiguration(reloadCtx, client, generation.ConfigPath)
			cancel()
			if err != nil {
				return fmt.Errorf("enable TUN configuration: %w", err)
			}
		}
		healthCtx, healthCancel := context.WithTimeout(ctx, 12*time.Second)
		err = client.WaitHealthy(healthCtx)
		healthCancel()
		if err != nil {
			return err
		}
		if err := m.restoreProxySelections(ctx, client, generation); err != nil {
			return err
		}
		if network, ok := m.driver.(managedNetwork); ok {
			if err := network.ActivateTUN(ctx, generation.Preferences); err != nil {
				return err
			}
		}
		deviceName := m.tunDevice()
		if deviceName != "" {
			observeCtx, observeCancel := context.WithTimeout(ctx, 15*time.Second)
			err = m.observeTUN(observeCtx, deviceName)
			observeCancel()
			if err != nil {
				return fmt.Errorf("verify TUN interface %s: %w", deviceName, err)
			}
		}
		return nil
	}
	if err := m.restoreProxySelections(ctx, client, generation); err != nil {
		return err
	}
	if err := m.systemProxy.Apply(ctx, generation.Preferences); err != nil {
		return fmt.Errorf("enable system proxy: %w", err)
	}
	return nil
}

func (m *Manager) stopCurrentLocked(ctx context.Context, reportStopped bool) error {
	m.mu.RLock()
	process := m.process
	client := m.client
	active := m.active
	captureSelections := m.status.State == StateRunning
	m.mu.RUnlock()
	if process == nil {
		if err := m.systemProxy.Restore(ctx); err != nil {
			return err
		}
		if active != nil && active.Session.Secret != "" {
			if err := m.store.EraseSessionFiles(active.Session); err != nil {
				return err
			}
			m.mu.Lock()
			if m.active == active {
				m.active.Session = ControllerSession{}
			}
			m.mu.Unlock()
		}
		if reportStopped {
			m.resetStopped()
		}
		return nil
	}
	m.setState(StateStopping)
	var firstErr error
	if network, ok := m.driver.(managedNetwork); ok {
		firstErr = network.DeactivateNetwork(ctx)
	}
	if active != nil && active.Preferences.ProxyMode == runtimeconfig.ProxyModeTUN && client != nil {
		drainCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if err := client.SetTUN(drainCtx, false); err != nil {
			firstErr = err
		}
		cancel()
		timer := time.NewTimer(800 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
		}
	}
	if err := m.systemProxy.Restore(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	// A failed reload/rollback may leave a different configuration in the
	// core. Never save those unconfirmed choices under the previous source.
	if captureSelections && active != nil && client != nil && !processExited(process) && ctx.Err() == nil {
		if err := m.captureProxySelections(ctx, client, *active); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	stopCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	if err := process.Stop(stopCtx); err != nil && firstErr == nil {
		firstErr = err
	}
	cancel()
	if active != nil && active.Session.Secret != "" {
		if err := m.store.EraseSessionFiles(active.Session); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	m.mu.Lock()
	if m.process == process {
		m.process = nil
	}
	if m.recoveryBlocked == process {
		m.recoveryBlocked = nil
	}
	if active != nil && m.active == active {
		m.active.Session = ControllerSession{}
	}
	m.client = nil
	m.status.PID = 0
	m.status.ControllerReady = false
	m.status.ControllerSession = nil
	m.status.SystemProxy = false
	m.status.TUNEnabled = false
	if reportStopped {
		desired := m.status.DesiredRunning
		m.status = Status{
			State: StateStopped, DesiredRunning: desired, ProxyMode: "off",
			CoreVersion: m.status.CoreVersion, GenerationID: m.status.GenerationID,
			SubscriptionID: m.status.SubscriptionID,
			Source:         m.status.Source,
			OutboundMode:   m.status.OutboundMode,
		}
	}
	m.mu.Unlock()
	return firstErr
}

func (m *Manager) watch(process Process) {
	go func() {
		<-process.Done()
		// Crash cleanup and user-triggered start/stop/reload must not interleave:
		// in particular, a late restore must never disable a newly applied
		// system proxy. Intentional Stop already owns this lock; once it releases
		// it has detached the process and this watcher becomes a no-op.
		m.operationMu.Lock()
		defer m.operationMu.Unlock()
		m.processExitedLocked(process)
	}()
}

// processExitedLocked also handles a confirmed service exit before its polling
// watcher observes Done. Callers must hold operationMu.
func (m *Manager) processExitedLocked(process Process) {
	m.mu.Lock()
	if m.process != process {
		m.mu.Unlock()
		return
	}
	m.process = nil
	m.client = nil
	m.status.PID = 0
	m.status.ControllerReady = false
	m.status.ControllerSession = nil
	m.status.SystemProxy = false
	m.status.TUNEnabled = false
	desired := m.status.DesiredRunning
	if m.recoveryBlocked == process {
		desired = false
		m.status.DesiredRunning = false
		m.recoveryBlocked = nil
	}
	var recoveryGeneration *Generation
	if m.active != nil {
		copy := *m.active
		recoveryGeneration = &copy
	}
	restartAttempt := m.status.RestartAttempt
	if startedAt, err := time.Parse(time.RFC3339Nano, m.status.StartedAt); err == nil && m.now().Sub(startedAt) >= time.Minute {
		restartAttempt = 0
	}
	if desired {
		m.status.State = StateRecovering
	} else {
		m.status.State = StateStopped
	}
	m.mu.Unlock()
	if err := m.systemProxy.Restore(context.Background()); err != nil {
		if recoveryGeneration != nil {
			if cleanupErr := m.store.EraseSessionFiles(recoveryGeneration.Session); cleanupErr != nil {
				err = fmt.Errorf("%v; erase failed runtime session: %w", err, cleanupErr)
			}
		}
		m.mu.Lock()
		m.status.DesiredRunning = false
		m.status.State = StateFailed
		m.status.LastError = &Failure{Code: "proxy_restore_failed", Phase: "recover", Message: safeErrorMessage(err)}
		if recoveryGeneration != nil && m.active != nil && m.active.Session.ID == recoveryGeneration.Session.ID {
			m.active.Session = ControllerSession{}
		}
		m.mu.Unlock()
		return
	}
	if desired && recoveryGeneration != nil {
		go m.recover(*recoveryGeneration, process.ExitError(), restartAttempt)
	} else if desired {
		m.mu.Lock()
		m.status.DesiredRunning = false
		m.status.State = StateFailed
		m.status.LastError = &Failure{Code: "generation_unavailable", Phase: "recover", Message: "active mihomo generation is unavailable"}
		m.mu.Unlock()
	} else if recoveryGeneration != nil {
		err := m.store.EraseSessionFiles(recoveryGeneration.Session)
		m.mu.Lock()
		if err != nil {
			m.status.State = StateFailed
			m.status.LastError = &Failure{Code: "session_cleanup_failed", Phase: "stop", Message: safeErrorMessage(err)}
		} else if m.active != nil && m.active.Session.ID == recoveryGeneration.Session.ID {
			m.active.Session = ControllerSession{}
		}
		m.mu.Unlock()
	}
}

func (m *Manager) deactivateNetwork(client *controllerClient, preferences runtimeconfig.Preferences) {
	if network, ok := m.driver.(managedNetwork); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = network.DeactivateNetwork(ctx)
		cancel()
	}
	if preferences.ProxyMode == runtimeconfig.ProxyModeSystemProxy {
		_ = m.systemProxy.Restore(context.Background())
		return
	}
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = client.SetTUN(ctx, false)
	cancel()
}

func (m *Manager) recover(generation Generation, exitErr error, attemptOffset int) {
	lastErr := exitErr
	for index := attemptOffset; index < len(m.restartDelays); index++ {
		delay := m.restartDelays[index]
		timer := time.NewTimer(delay)
		select {
		case <-m.shutdown:
			timer.Stop()
			return
		case <-timer.C:
		}
		m.mu.RLock()
		desired := m.status.DesiredRunning
		m.mu.RUnlock()
		if !desired {
			return
		}
		m.operationMu.Lock()
		m.mu.RLock()
		canStart := m.status.DesiredRunning && m.process == nil
		m.mu.RUnlock()
		if !canStart {
			m.operationMu.Unlock()
			return
		}
		var err error
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		err = m.launchLocked(recoveryCtx, generation, index+1)
		cancel()
		m.operationMu.Unlock()
		if err == nil {
			return
		}
		lastErr = err
	}
	cleanupErr := m.store.EraseSessionFiles(generation.Session)
	m.mu.Lock()
	if !m.status.DesiredRunning || m.process != nil {
		m.mu.Unlock()
		return
	}
	m.status.DesiredRunning = false
	m.status.State = StateFailed
	if cleanupErr != nil {
		lastErr = fmt.Errorf("%v; erase failed runtime session: %w", lastErr, cleanupErr)
	}
	m.status.LastError = &Failure{Code: requirements.Code(lastErr, "process_exited"), Phase: "recover", Message: safeExitMessage(lastErr)}
	if m.active != nil && m.active.Session.ID == generation.Session.ID {
		m.active.Session = ControllerSession{}
	}
	m.mu.Unlock()
}

func (m *Manager) runningClient() (*controllerClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.status.State != StateRunning || m.client == nil || !m.status.ControllerReady {
		return nil, fmt.Errorf("mihomo controller is not ready")
	}
	return m.client, nil
}

func (m *Manager) setState(state State) {
	m.mu.Lock()
	m.status.State = state
	m.mu.Unlock()
}

func (m *Manager) fail(code, phase string, err error) (Status, error) {
	m.mu.Lock()
	m.status.State = StateFailed
	m.status.LastError = &Failure{Code: requirements.Code(err, code), Phase: phase, Message: safeErrorMessage(err)}
	status := cloneStatus(m.status)
	m.mu.Unlock()
	return status, err
}

func (m *Manager) failRunning(code, phase string, err error) (Status, error) {
	m.mu.Lock()
	m.status.State = StateRunning
	m.status.LastError = &Failure{Code: requirements.Code(err, code), Phase: phase, Message: safeErrorMessage(err)}
	status := cloneStatus(m.status)
	m.mu.Unlock()
	return status, err
}

func (m *Manager) resetStopped() {
	m.mu.Lock()
	desired := m.status.DesiredRunning
	m.status = Status{
		State: StateStopped, DesiredRunning: desired, ProxyMode: "off",
		CoreVersion: m.status.CoreVersion, GenerationID: m.status.GenerationID,
		SubscriptionID: m.status.SubscriptionID,
		Source:         m.status.Source,
		OutboundMode:   m.status.OutboundMode,
	}
	m.mu.Unlock()
}

func cloneStatus(status Status) Status {
	copy := status
	copy.ControllerSession = cloneSession(status.ControllerSession)
	if status.LastError != nil {
		failure := *status.LastError
		copy.LastError = &failure
	}
	return copy
}

func cloneSession(session *ControllerSession) *ControllerSession {
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}

func validateStartRequest(request StartRequest) error {
	if strings.TrimSpace(request.ExecutablePath) == "" || strings.TrimSpace(request.CoreVersion) == "" {
		return fmt.Errorf("selected mihomo core is required")
	}
	if strings.TrimSpace(request.Source.SubscriptionID) == "" {
		return fmt.Errorf("selected subscription is required")
	}
	if len(request.Configuration) == 0 {
		return fmt.Errorf("final runtime configuration is empty")
	}
	return runtimeconfig.Validate(request.Preferences)
}

func sameExecutable(goos, left, right string) bool {
	if goos == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func safeErrorMessage(err error) string {
	if err == nil {
		return "runtime operation failed"
	}
	message := sanitizeDiagnostic(err.Error())
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}

func safeExitMessage(err error) string {
	if err == nil {
		return "mihomo exited unexpectedly"
	}
	return safeErrorMessage(err)
}

func waitForTUNInterface(ctx context.Context, name string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, item := range interfaces {
				if item.Name == name {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func processExited(process Process) bool {
	select {
	case <-process.Done():
		return true
	default:
		return false
	}
}

type unavailableSystemProxy struct{}

func (unavailableSystemProxy) Apply(context.Context, runtimeconfig.Preferences) error {
	return fmt.Errorf("system proxy controller is unavailable")
}
func (unavailableSystemProxy) Restore(context.Context) error { return nil }
func (unavailableSystemProxy) Recover(context.Context) error { return nil }
