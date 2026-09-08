package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"jeemi/internal/config/dnstool"
	"jeemi/internal/config/document"
)

type proxySelectionState struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

func (c *controllerClient) proxySelections(ctx context.Context) (map[string]proxySelectionState, error) {
	var result struct {
		Proxies map[string]proxySelectionState `json:"proxies"`
	}
	err := c.doJSONWithLimit(ctx, http.MethodGet, "/proxies", nil, &result, 16<<20)
	if err == nil && result.Proxies == nil {
		err = fmt.Errorf("mihomo returned no proxy state")
	}
	return result.Proxies, err
}

// RememberProxySelection persists a successful direct REST selection. The
// active session AND subscription are checked because a hot reload can reuse
// the same controller session for a different subscription.
func (m *Manager) RememberProxySelection(ctx context.Context, sessionID, subscriptionID, group, proxy string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	client, err := m.runningClient()
	if err != nil {
		return err
	}
	if m.active == nil || m.active.Session.ID != sessionID || m.active.Source.SubscriptionID != subscriptionID {
		return fmt.Errorf("proxy selection belongs to an expired runtime source")
	}
	return m.rememberProxySelection(ctx, client, group, proxy)
}

func (m *Manager) rememberProxySelection(ctx context.Context, client *controllerClient, group, proxy string) error {
	if strings.HasPrefix(group, dnstool.Prefix) {
		return nil
	}
	if m.forgottenSubscriptions[m.active.Source.SubscriptionID] {
		return fmt.Errorf("proxy selection subscription was deleted")
	}
	if !selectionNameValid(group) || !selectionNameValid(proxy) {
		return fmt.Errorf("invalid proxy selection")
	}
	groups, err := client.proxySelections(ctx)
	if err != nil {
		return err
	}
	state, found := groups[group]
	if !found {
		return fmt.Errorf("proxy selector is unavailable")
	}
	// URLTest, Fallback and LoadBalance must remain under mihomo's control.
	if state.Type != "Selector" {
		return nil
	}
	if state.Now != proxy || !slices.Contains(state.All, proxy) {
		return fmt.Errorf("proxy selection changed before it could be saved")
	}
	return m.selections.merge(m.active.Source.SubscriptionID, map[string]string{group: proxy})
}

func (m *Manager) captureProxySelections(ctx context.Context, client *controllerClient, generation Generation) error {
	if m.forgottenSubscriptions[generation.Source.SubscriptionID] {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	groups, err := client.proxySelections(ctx)
	if err != nil {
		return fmt.Errorf("read proxy selections before configuration change: %w", err)
	}
	values := map[string]string{}
	for name, state := range groups {
		if !strings.HasPrefix(name, dnstool.Prefix) && state.Type == "Selector" && slices.Contains(state.All, state.Now) {
			values[name] = state.Now
		}
	}
	return m.selections.merge(generation.Source.SubscriptionID, values)
}

// restoreProxySelections runs after the final configuration is loaded, before
// enabling system proxy / declaring network activation complete. Explicit
// defaults also prevent Linux's shared mihomo cache from leaking choices
// between subscriptions that happen to use the same group names.
func (m *Manager) restoreProxySelections(ctx context.Context, client *controllerClient, generation Generation) error {
	saved, err := m.selections.load(generation.Source.SubscriptionID)
	if err != nil {
		return err
	}
	defaults, err := selectorDefaults(generation.ConfigPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	groups, err := client.proxySelections(ctx)
	if err != nil {
		return fmt.Errorf("read proxy selectors for restoration: %w", err)
	}
	names := make([]string, 0, len(groups))
	for name, state := range groups {
		if !strings.HasPrefix(name, dnstool.Prefix) && state.Type == "Selector" && len(state.All) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		state := groups[name]
		chosen := saved[name]
		if !slices.Contains(state.All, chosen) {
			chosen = defaults[name]
			if !slices.Contains(state.All, chosen) {
				chosen = state.All[0]
			}
		}
		if state.Now != chosen {
			if err := client.SelectProxy(ctx, name, chosen); err != nil {
				return fmt.Errorf("restore proxy selection: %w", err)
			}
		}
	}
	return nil
}

func selectorDefaults(configuration string) (map[string]string, error) {
	data, err := os.ReadFile(configuration)
	if err != nil {
		return nil, fmt.Errorf("cannot read proxy selector defaults")
	}
	parsed, err := document.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy selector defaults")
	}
	var config struct {
		Groups []struct {
			Name    string `yaml:"name"`
			Type    string `yaml:"type"`
			Default string `yaml:"default-selected"`
		} `yaml:"proxy-groups"`
	}
	if err := parsed.Decode(&config); err != nil {
		return nil, fmt.Errorf("invalid proxy selector defaults")
	}
	result := map[string]string{}
	for _, group := range config.Groups {
		if group.Type == "select" {
			result[group.Name] = group.Default
		}
	}
	return result, nil
}

func (m *Manager) ForgetProxySelections(subscriptionID string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if _, err := m.selections.path(subscriptionID); err != nil {
		return err
	}
	m.forgottenSubscriptions[subscriptionID] = true
	return m.selections.delete(subscriptionID)
}
