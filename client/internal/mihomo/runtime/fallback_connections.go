package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sync"
	"sync/atomic"

	"jeemi/internal/config/fallbackoverride"
	"jeemi/internal/runtimeconfig"
)

// FallbackConnectionReset records the old route for the application transaction.
// Controller credentials and connection details never cross Wails.
type FallbackConnectionReset struct {
	sessionID      string
	generationID   string
	subscriptionID string
	target         string
	previousTarget string
	mode           string
	err            error
}

// PrepareFallbackConnectionReset captures the actual old target, not a saved
// preference that may have failed to apply. It never reads or closes connections.
func (m *Manager) PrepareFallbackConnectionReset(subscriptionID string, configuration []byte, mode string) *FallbackConnectionReset {
	if mode != "selector" && mode != "all" {
		return nil
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.mu.RLock()
	active, client := m.active, m.client
	ready := m.status.State == StateRunning && m.status.ControllerReady &&
		active != nil && client != nil && active.Source.SubscriptionID == subscriptionID &&
		active.Preferences.OutboundMode == runtimeconfig.OutboundModeRule
	m.mu.RUnlock()
	if !ready {
		return nil
	}
	plan := &FallbackConnectionReset{
		sessionID: active.Session.ID, generationID: active.ID, subscriptionID: subscriptionID,
		mode: mode,
	}
	_, next, err := fallbackoverride.Apply(configuration, fallbackoverride.Selection{Mode: fallbackoverride.ModeNone})
	if err != nil {
		plan.err = err
		return plan
	}
	plan.target = next.OriginalTarget
	contents, err := os.ReadFile(active.ConfigPath)
	if err != nil {
		plan.err = err
		return plan
	}
	_, previous, err := fallbackoverride.Apply(contents, fallbackoverride.Selection{Mode: fallbackoverride.ModeNone})
	if err != nil {
		plan.err = err
		return plan
	}
	if previous.OriginalTarget == plan.target || (mode == "selector" && previous.OriginalTarget == "") {
		return nil
	}
	plan.previousTarget = previous.OriginalTarget
	return plan
}

// CompleteFallbackConnectionReset only operates on a healthy replacement of the
// captured generation in the same controller session. A restart already retired
// those connections; a failed/deferred reload must leave them alone.
func (m *Manager) CompleteFallbackConnectionReset(ctx context.Context, plan *FallbackConnectionReset, revision int) error {
	if plan == nil {
		return nil
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.mu.RLock()
	active, client := m.active, m.client
	ready := m.status.State == StateRunning && m.status.ControllerReady &&
		active != nil && client != nil && active.ID != plan.generationID &&
		active.Session.ID == plan.sessionID && active.Source.SubscriptionID == plan.subscriptionID &&
		active.Source.FallbackOverrideRevision == revision &&
		active.Preferences.OutboundMode == runtimeconfig.OutboundModeRule
	m.mu.RUnlock()
	if !ready {
		return nil
	}
	if plan.err != nil {
		return fmt.Errorf("fallback changed but the previous route could not be captured")
	}
	contents, err := os.ReadFile(active.ConfigPath)
	if err != nil {
		return fmt.Errorf("fallback connection reset could not verify active configuration")
	}
	_, current, err := fallbackoverride.Apply(contents, fallbackoverride.Selection{Mode: fallbackoverride.ModeNone})
	if err != nil || current.OriginalTarget != plan.target {
		return fmt.Errorf("fallback connection reset target no longer matches active configuration")
	}
	var snapshot struct {
		Connections []struct {
			ID     string   `json:"id"`
			Chains []string `json:"chains"`
		} `json:"connections"`
	}
	// Fetch after activation, just like node switching: connections established
	// on the old route during validation/reload must also be included. The larger
	// bound accommodates metadata for many live sockets without unbounded input.
	if err := client.doJSONWithLimit(ctx, http.MethodGet, "/connections", nil, &snapshot, 16<<20); err != nil {
		return fmt.Errorf("fallback changed but current connections could not be read")
	}
	var ids []string
	seen := make(map[string]bool)
	for _, connection := range snapshot.Connections {
		if connection.ID == "" || seen[connection.ID] {
			continue
		}
		if plan.mode == "all" || slices.Contains(connection.Chains, plan.previousTarget) {
			ids = append(ids, connection.ID)
			seen[connection.ID] = true
		}
	}
	var failed atomic.Bool
	var next atomic.Int64
	var workers sync.WaitGroup
	for range min(8, len(ids)) {
		workers.Go(func() {
			for {
				index := int(next.Add(1) - 1)
				if index >= len(ids) {
					return
				}
				if err := client.doJSON(ctx, http.MethodDelete, "/connections/"+url.PathEscape(ids[index]), nil, nil); err != nil {
					failed.Store(true)
				}
			}
		})
	}
	workers.Wait()
	if failed.Load() {
		return fmt.Errorf("fallback changed but some previous connections could not be reset")
	}
	return nil
}
