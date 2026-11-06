package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/runtimeconfig"
	"jeemi/internal/subscription"
)

type scheduledFetcher struct {
	calls    atomic.Int32
	contents string
	err      error
	started  chan struct{}
}

func (f *scheduledFetcher) Fetch(ctx context.Context, _ string) (subscription.FetchedContent, error) {
	f.calls.Add(1)
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return subscription.FetchedContent{}, ctx.Err()
	}
	return subscription.FetchedContent{Contents: []byte(f.contents), Format: subscription.FormatYAML}, f.err
}

func scheduledService(t *testing.T) (*Service, *scheduledFetcher, *time.Time, string) {
	t.Helper()
	directory := t.TempDir()
	s, err := NewService(directory)
	if err != nil {
		t.Fatal(err)
	}
	useDomainOnlyFakeIPFilter(t, s)
	now := time.Now().UTC().Add(-24 * time.Hour)
	store, err := subscription.NewStore(subscription.StoreOptions{DataDirectory: directory, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	f := &scheduledFetcher{contents: fallbackFixture}
	s.subscriptions, err = subscription.NewManager(subscription.ManagerOptions{Store: store, Fetcher: f})
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/subscription"})
	if err != nil {
		t.Fatal(err)
	}
	return s, f, &now, state.SelectedSubscriptionID
}

func TestAutomaticSubscriptionUpdateIntervalsAndCatchup(t *testing.T) {
	s, f, now, first := scheduledService(t)
	state, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Second", SourceURL: "https://example.test/second", AutoUpdateIntervalMinutes: 30})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "fixture.yaml")
	if err := os.WriteFile(file, []byte(fallbackFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportSubscriptionFile(file); err != nil {
		t.Fatal(err)
	}
	if state.SelectedSubscriptionID != first {
		t.Fatal("import changed selection")
	}
	var second string
	for _, item := range state.Subscriptions {
		if item.ID != first {
			second = item.ID
		}
	}
	before, err := s.Subscription(second)
	if err != nil {
		t.Fatal(err)
	}
	var notifications atomic.Int32
	s.SetSubscriptionUpdateNotifier(func() { notifications.Add(1) })
	*now = now.Add(29 * time.Minute)
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 2 {
		t.Fatal("fetched before interval")
	}
	*now = now.Add(time.Minute)
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 2 {
		t.Fatal("unselected 30 minute subscription was refreshed")
	}
	*now = now.Add(30 * time.Minute)
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 3 {
		t.Fatal("selected hourly subscription did not refresh alone")
	}
	*now = now.Add(24 * time.Hour)
	s.updateDueSubscriptions(context.Background(), *now)
	s.updateDueSubscriptions(context.Background(), now.Add(15*time.Second))
	if f.calls.Load() != 4 || notifications.Load() != 2 {
		t.Fatal("missed intervals replayed or file subscription refreshed")
	}
	unselected, err := s.Subscription(second)
	if err != nil || !reflect.DeepEqual(unselected, before) {
		t.Fatal("unselected subscription metadata changed", err)
	}
	after, err := s.SubscriptionState()
	if err != nil || after.SelectedSubscriptionID != first {
		t.Fatal("background update changed selection", err)
	}
	if s.runtimeManager.Status().State == "running" {
		t.Fatal("automatic subscription updates started the core")
	}
}

func TestAutomaticSubscriptionUpdateRechecksSelectionAndExcludesFiles(t *testing.T) {
	s, f, now, first := scheduledService(t)
	state, err := s.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Second", SourceURL: "https://example.test/second", AutoUpdateIntervalMinutes: 30})
	if err != nil {
		t.Fatal(err)
	}
	var second string
	for _, item := range state.Subscriptions {
		if item.ID != first {
			second = item.ID
		}
	}
	before, err := s.Subscription(first)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(2 * time.Hour)
	if _, err := s.SelectSubscription(second); err != nil {
		t.Fatal(err)
	}
	// The old selection was captured before a foreground switch acquired the lock.
	s.reconcileMu.Lock()
	changed := s.updateDueSubscriptionLocked(context.Background(), first, *now)
	s.reconcileMu.Unlock()
	if changed || f.calls.Load() != 2 || len(s.autoUpdateAttempts) != 0 {
		t.Fatal("stale selection fetched an inactive subscription")
	}
	s.updateDueSubscriptions(context.Background(), *now)
	s.updateDueSubscriptions(context.Background(), now.Add(15*time.Second))
	if f.calls.Load() != 3 {
		t.Fatal("new selection must catch up once using its original fetch time")
	}
	after, err := s.Subscription(first)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("switching changed inactive metadata", err)
	}
	file := filepath.Join(t.TempDir(), "fixture.yaml")
	if err := os.WriteFile(file, []byte(fallbackFixture), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = s.ImportSubscriptionFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var fileID string
	for _, item := range state.Subscriptions {
		if item.SourceKind == subscription.SourceFile {
			fileID = item.ID
		}
	}
	if _, err := s.SelectSubscription(fileID); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(24 * time.Hour)
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 3 {
		t.Fatal("file selection refreshed another URL subscription")
	}
	// Manual refresh remains available for subscriptions outside the selection.
	if state, err := s.RefreshSubscription(first); err != nil || state.SelectedSubscriptionID != fileID || f.calls.Load() != 4 {
		t.Fatal("manual refresh of an inactive subscription was blocked or changed selection", err)
	}
	if _, err := s.settingsStore.SetSelectedSubscription(""); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(24 * time.Hour)
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 4 {
		t.Fatal("empty selection refreshed a subscription")
	}
}

func TestAutomaticSubscriptionUpdateFailuresRetainRevisionAndAssociation(t *testing.T) {
	for _, mode := range []string{"fetch", "invalid", "handler"} {
		t.Run(mode, func(t *testing.T) {
			s, f, now, id := scheduledService(t)
			if mode == "handler" {
				script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Conditional", Contents: "function main(config) { if (config.newSource) config['proxy-groups'] = {}; return config; }"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := s.Subscription(id)
			switch mode {
			case "fetch":
				f.err = errors.New("private transport failure")
			case "invalid":
				f.contents = "proxy-groups: {}\n"
			case "handler":
				f.contents = fallbackFixture + "newSource: true\n"
			}
			*now = now.Add(time.Hour)
			s.updateDueSubscriptions(context.Background(), *now)
			after, _ := s.Subscription(id)
			if !reflect.DeepEqual(before, after) || s.pendingRefresh != nil {
				t.Fatal("automatic failure changed persisted input or created a detach confirmation")
			}
			state, err := s.SubscriptionState()
			if err != nil || state.Subscriptions[0].AutoUpdateError == "" {
				t.Fatal("automatic failure was not visible", err)
			}
			s.updateDueSubscriptions(context.Background(), now.Add(15*time.Second))
			if f.calls.Load() != 2 {
				t.Fatal("failed update retried too soon")
			}
			f.err, f.contents = nil, fallbackFixture+"newSource: false\n"
			*now = now.Add(time.Hour)
			s.updateDueSubscriptions(context.Background(), *now)
			state, err = s.SubscriptionState()
			if err != nil || state.Subscriptions[0].AutoUpdateError != "" || f.calls.Load() != 3 {
				t.Fatal("next interval did not recover", err)
			}
		})
	}
}

func TestAutomaticSubscriptionUpdateRespectsForegroundAndManualRefresh(t *testing.T) {
	s, f, now, id := scheduledService(t)
	*now = now.Add(time.Hour)
	s.reconcileMu.Lock()
	s.updateDueSubscriptions(context.Background(), *now)
	s.reconcileMu.Unlock()
	if f.calls.Load() != 1 {
		t.Fatal("overlapped a foreground transaction")
	}
	if _, err := s.RefreshSubscription(id); err != nil {
		t.Fatal(err)
	}
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 2 {
		t.Fatal("manual success did not reset the interval")
	}
	*now = now.Add(time.Hour)
	before, _ := s.Subscription(id)
	s.pendingRefresh = &pendingSubscriptionRefresh{token: "fixture", expires: now.Add(10 * time.Minute), candidate: subscription.RefreshCandidate{Before: before}}
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 2 || s.pendingRefresh.token != "fixture" {
		t.Fatal("automatic update invalidated the manual confirmation")
	}
	s.pendingRefresh = nil
	if _, err := s.DeleteSubscription(id); err != nil {
		t.Fatal(err)
	}
	s.updateDueSubscriptions(context.Background(), *now)
	if f.calls.Load() != 2 || len(s.autoUpdateAttempts) != 0 {
		t.Fatal("deleted subscription was refreshed")
	}
}

func TestAutomaticSubscriptionUpdateLifecycleCancelsAndStartsOnlyOnce(t *testing.T) {
	s, f, _, id := scheduledService(t)
	before, _ := s.Subscription(id)
	f.started = make(chan struct{}, 2)
	s.startSubscriptionAutoUpdates(context.Background())
	s.startSubscriptionAutoUpdates(context.Background())
	select {
	case <-f.started:
	case <-time.After(2 * time.Second):
		t.Fatal("overdue subscription was not fetched on startup")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.stopSubscriptionAutoUpdates(ctx)
	if ctx.Err() != nil || f.calls.Load() != 2 {
		t.Fatal("scheduler did not cancel cleanly or started twice")
	}
	after, _ := s.Subscription(id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("cancelled fetch changed subscription")
	}
}

type cancellingUpdateDriver struct {
	authorizationDriver
	validations atomic.Int32
	blocked     chan struct{}
}

func (d *cancellingUpdateDriver) Validate(ctx context.Context, _, _, _ string) error {
	if d.validations.Add(1) == 1 {
		return nil // Accept the subscription candidate before committing it.
	}
	close(d.blocked)
	<-ctx.Done()
	return ctx.Err()
}

func TestAutomaticSubscriptionUpdateCancellationReachesReconciliation(t *testing.T) {
	s, _ := authorizationService(t, runtimeconfig.ProxyModeSystemProxy)
	serviceContext, stopService := context.WithCancel(context.Background())
	defer stopService()
	s.ctx = serviceContext
	driver := &cancellingUpdateDriver{blocked: make(chan struct{})}
	var err error
	s.runtimeManager, err = mihomoruntime.NewManager(mihomoruntime.Options{
		DataDirectory: filepath.Dir(s.localConfigs.Directory()),
		Driver:        driver, SystemProxy: fallbackTestSystemProxy{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.updateDueSubscriptions(ctx, time.Now().Add(2*time.Hour))
	}()
	select {
	case <-driver.blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("automatic update did not reach configuration reconciliation")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("configuration reconciliation ignored automatic update cancellation")
	}
	if driver.starts.Load() != 0 {
		t.Fatal("automatic update started a stopped core")
	}
}
