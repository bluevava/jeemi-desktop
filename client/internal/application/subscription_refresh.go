package application

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"jeemi/internal/subscription"
)

type SubscriptionRefreshConflict struct {
	Token          string `json:"token"`
	SubscriptionID string `json:"subscriptionId"`
	LocalConfigID  string `json:"localConfigId"`
	LocalScriptID  string `json:"localScriptId"`
	Message        string `json:"message"`
}

type SubscriptionRefreshResult struct {
	State    *subscription.State          `json:"state"`
	Conflict *SubscriptionRefreshConflict `json:"conflict"`
}

type pendingSubscriptionRefresh struct {
	token     string
	expires   time.Time
	candidate subscription.RefreshCandidate
}

// CheckSubscriptionRefresh validates the fetched bytes before any revision or
// metadata is written. At most one bounded, expiring candidate is retained.
func (s *Service) CheckSubscriptionRefresh(id string) (SubscriptionRefreshResult, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.pendingRefresh = nil
	ctx, cancel := s.operationContext(60 * time.Second)
	defer cancel()
	candidate, err := s.subscriptions.PrepareRefresh(ctx, id)
	if err != nil {
		return SubscriptionRefreshResult{}, err
	}
	summary := candidate.Before.Summary
	_, combinedErr := s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{source: candidate.Fetched.Contents})
	if combinedErr == nil {
		return s.commitSubscriptionRefresh(candidate, false)
	}
	if summary.LocalConfigID == "" && summary.LocalScriptID == "" {
		return SubscriptionRefreshResult{}, candidateValidationError(summary, combinedErr)
	}
	// A detach offer is valid only if the same fetched source works without
	// the local handler. A broken source must never replace the old revision.
	summary.LocalConfigID, summary.LocalScriptID = "", ""
	if _, err := s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{source: candidate.Fetched.Contents}); err != nil {
		return SubscriptionRefreshResult{}, candidateValidationError(summary, err)
	}
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return SubscriptionRefreshResult{}, fmt.Errorf("create refresh confirmation")
	}
	token := hex.EncodeToString(tokenBytes)
	s.pendingRefresh = &pendingSubscriptionRefresh{token: token, expires: time.Now().Add(10 * time.Minute), candidate: candidate}
	return SubscriptionRefreshResult{Conflict: &SubscriptionRefreshConflict{
		Token: token, SubscriptionID: id, LocalConfigID: candidate.Before.LocalConfigID,
		LocalScriptID: candidate.Before.LocalScriptID, Message: safeConfigurationError(combinedErr),
	}}, nil
}

// ResolveSubscriptionRefresh accepts or discards exactly the checked bytes.
// Revalidation and optimistic metadata comparison prevent a stale dialog from
// overwriting intervening edits. Detachment and revision commit are atomic.
func (s *Service) ResolveSubscriptionRefresh(token string, detach bool) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	pending := s.pendingRefresh
	if pending == nil || pending.token != token {
		return subscription.State{}, fmt.Errorf("refresh confirmation is no longer available")
	}
	s.pendingRefresh = nil
	if !detach {
		return s.SubscriptionState()
	}
	if time.Now().After(pending.expires) {
		return subscription.State{}, fmt.Errorf("refresh confirmation expired; refresh again")
	}
	summary := pending.candidate.Before.Summary
	summary.LocalConfigID, summary.LocalScriptID = "", ""
	ctx, cancel := s.operationContext(45 * time.Second)
	defer cancel()
	if _, err := s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{source: pending.candidate.Fetched.Contents}); err != nil {
		return subscription.State{}, candidateValidationError(summary, err)
	}
	result, err := s.commitSubscriptionRefresh(pending.candidate, true)
	if err != nil {
		return subscription.State{}, err
	}
	return *result.State, nil
}

func (s *Service) commitSubscriptionRefresh(candidate subscription.RefreshCandidate, detach bool) (SubscriptionRefreshResult, error) {
	if _, err := s.subscriptions.CommitRefresh(candidate, detach); err != nil {
		return SubscriptionRefreshResult{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerSubscription, nil, reconcileAutomatic)
	state, err := s.SubscriptionState()
	if err != nil {
		return SubscriptionRefreshResult{}, err
	}
	return SubscriptionRefreshResult{State: &state}, nil
}
