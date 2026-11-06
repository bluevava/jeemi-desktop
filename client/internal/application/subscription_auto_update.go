package application

import (
	"context"
	"time"

	"jeemi/internal/subscription"
)

type subscriptionUpdateAttempt struct {
	at        time.Time
	errorCode string
}

// SetSubscriptionUpdateNotifier carries an invalidation only, never source URLs
// or subscription contents. The desktop sets it before Startup.
func (s *Service) SetSubscriptionUpdateNotifier(notify func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoUpdateNotify = notify
}

func (s *Service) startSubscriptionAutoUpdates(parent context.Context) {
	s.mu.Lock()
	if s.autoUpdateCancel != nil || s.shuttingDown {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	s.autoUpdateCancel, s.autoUpdateDone = cancel, done
	s.mu.Unlock()
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			s.updateDueSubscriptions(ctx, time.Now())
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) stopSubscriptionAutoUpdates(ctx context.Context) {
	s.mu.RLock()
	cancel, done := s.autoUpdateCancel, s.autoUpdateDone
	s.mu.RUnlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (s *Service) subscriptionUpdateDue(item subscription.Summary, now time.Time) bool {
	if item.SourceKind != subscription.SourceURL || item.AutoUpdateIntervalMinutes <= 0 {
		return false
	}
	last, err := time.Parse(time.RFC3339Nano, item.LastFetchedAt)
	if err != nil {
		return false
	}
	s.mu.RLock()
	attempt := s.autoUpdateAttempts[item.ID]
	stopping := s.shuttingDown
	s.mu.RUnlock()
	if stopping {
		return false
	}
	if attempt.at.After(last) {
		last = attempt.at
	}
	return !now.Before(last.Add(time.Duration(item.AutoUpdateIntervalMinutes) * time.Minute))
}

// This loop stays independent of renderer visibility and core running state.
// Only the selected URL subscription participates in unattended updates.
// Never wait for a foreground configuration operation: recheck on the next tick.
func (s *Service) updateDueSubscriptions(ctx context.Context, now time.Time) {
	if ctx.Err() != nil {
		return
	}
	state, err := s.subscriptions.State()
	if err != nil {
		return
	}
	s.mu.Lock()
	for id := range s.autoUpdateAttempts {
		found := false
		for _, item := range state.Subscriptions {
			found = found || item.ID == id
		}
		if !found {
			delete(s.autoUpdateAttempts, id)
		}
	}
	s.mu.Unlock()
	document, err := s.settingsStore.Load()
	if err != nil {
		return
	}
	for _, item := range state.Subscriptions {
		if ctx.Err() != nil || item.ID != document.Subscriptions.SelectedID || !s.subscriptionUpdateDue(item, now) {
			continue
		}
		if !s.reconcileMu.TryLock() {
			return
		}
		changed := s.updateDueSubscriptionLocked(ctx, item.ID, now)
		s.reconcileMu.Unlock()
		if changed {
			s.mu.RLock()
			notify := s.autoUpdateNotify
			s.mu.RUnlock()
			if notify != nil && ctx.Err() == nil {
				notify()
			}
		}
	}
}

func (s *Service) updateDueSubscriptionLocked(parent context.Context, id string, now time.Time) bool {
	// Selection can change after the initial scan. Holding reconcileMu keeps the
	// selected subscription stable throughout fetching, validation and commit.
	document, err := s.settingsStore.Load()
	if err != nil || id == "" || document.Subscriptions.SelectedID != id {
		return false
	}
	item, err := s.subscriptions.Get(id)
	if err != nil || parent.Err() != nil || !s.subscriptionUpdateDue(item.Summary, now) {
		return false
	}
	// An unattended update must neither invalidate a confirmation nor detach its
	// handler. The user can resolve the existing manual candidate first.
	if pending := s.pendingRefresh; pending != nil && pending.candidate.Before.ID == id && now.Before(pending.expires) {
		return false
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	candidate, err := s.subscriptions.PrepareRefresh(ctx, id)
	errorCode := "refresh_failed"
	if err == nil {
		errorCode = "validation_failed"
		_, err = s.validateSubscriptionCandidate(ctx, candidate.Before.Summary, compositionCandidate{source: candidate.Fetched.Contents})
	}
	if ctx.Err() != nil {
		if parent.Err() != nil {
			return false
		}
		err = ctx.Err()
	}
	if err == nil {
		errorCode = "refresh_failed"
		_, err = s.subscriptions.CommitRefresh(candidate, false)
		if err == nil {
			errorCode = ""
			reconcileContext, reconcileCancel := context.WithTimeout(parent, 100*time.Second)
			_ = s.reconcileSelectedLocked(reconcileContext, reconcileTriggerSubscription, nil, reconcileAutomatic)
			reconcileCancel()
		}
	}
	s.mu.Lock()
	if s.autoUpdateAttempts == nil {
		s.autoUpdateAttempts = make(map[string]subscriptionUpdateAttempt)
	}
	s.autoUpdateAttempts[id] = subscriptionUpdateAttempt{at: now, errorCode: errorCode}
	s.mu.Unlock()
	return true
}

func (s *Service) decorateSubscriptionUpdates(state *subscription.State) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range state.Subscriptions {
		item := &state.Subscriptions[i]
		attempt := s.autoUpdateAttempts[item.ID]
		last, _ := time.Parse(time.RFC3339Nano, item.LastFetchedAt)
		if !last.After(attempt.at) {
			item.AutoUpdateError = attempt.errorCode
		}
	}
}
