package application

import (
	"context"
	"fmt"
	"time"

	"jeemi/internal/config/runtimecontrol"
	"jeemi/internal/localscript"
	"jeemi/internal/subscription"
)

func (s *Service) LocalScriptState() (localscript.State, error) {
	return s.localScripts.State()
}

func (s *Service) LocalScript(id string) (localscript.Script, error) {
	return s.localScripts.Get(id)
}

func (s *Service) TestLocalScript(input localscript.TestInput) (localscript.TestResult, error) {
	ctx, cancel := s.operationContext(60 * time.Second)
	defer cancel()
	prepared, err := s.prepareLocalScriptInput(ctx, input.SaveInput, false)
	if err != nil {
		return localscript.TestResult{}, err
	}
	candidate, err := s.localScripts.Preview(prepared)
	if err != nil {
		return localscript.TestResult{}, err
	}
	target, err := s.subscriptionSummary(input.SubscriptionID)
	if err != nil {
		return localscript.TestResult{}, err
	}
	result, err := s.validateLocalScriptCandidate(ctx, target, candidate)
	if err != nil {
		return localscript.TestResult{}, err
	}
	displayContents, err := runtimecontrol.FormatForDisplay([]byte(result.Contents))
	if err != nil {
		return localscript.TestResult{}, fmt.Errorf("format local script test result for display: %w", err)
	}
	result.Contents = string(displayContents)
	return result, nil
}

func (s *Service) SaveLocalScript(input localscript.SaveInput) (localscript.Script, error) {
	return s.saveLocalScript(input, false)
}

func (s *Service) saveLocalScript(input localscript.SaveInput, forceDownload bool) (localscript.Script, error) {
	ctx, cancel := s.operationContext(75 * time.Second)
	defer cancel()
	input, err := s.prepareLocalScriptInput(ctx, input, forceDownload)
	if err != nil {
		return localscript.Script{}, err
	}
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return localscript.Script{}, err
	}

	candidate, err := s.localScripts.Preview(input)
	if err != nil {
		return localscript.Script{}, err
	}
	references := []subscription.Summary{}
	if input.ID != "" {
		references, err = s.subscriptions.ReferencingLocalScript(input.ID)
		if err != nil {
			return localscript.Script{}, err
		}
	}
	for _, reference := range references {
		if _, err := s.validateLocalScriptCandidate(ctx, reference, candidate); err != nil {
			return localscript.Script{}, fmt.Errorf("local script would make subscription %q invalid: %w", reference.Name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return localscript.Script{}, err
	}
	saved, err := s.localScripts.Save(input)
	if err != nil {
		return localscript.Script{}, err
	}
	if len(references) > 0 {
		_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalScript, nil, reconcileAutomatic)
	}
	return saved, nil
}

func (s *Service) DeleteLocalScript(id string) (localscript.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	references, err := s.subscriptions.ReferencingLocalScript(id)
	if err != nil {
		return localscript.State{}, err
	}
	if len(references) > 0 {
		return localscript.State{}, fmt.Errorf("local script is still associated with %d subscription(s)", len(references))
	}
	return s.localScripts.Delete(id)
}

func (s *Service) SetSubscriptionLocalScript(id, localScriptID string) (subscription.State, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.associationMu.Lock()
	defer s.associationMu.Unlock()
	detail, err := s.subscriptions.Get(id)
	if err != nil {
		return subscription.State{}, err
	}
	if localScriptID != "" {
		if detail.LocalConfigID != "" {
			return subscription.State{}, fmt.Errorf("unlink the local configuration before associating a local script")
		}
		script, err := s.localScripts.Get(localScriptID)
		if err != nil {
			return subscription.State{}, fmt.Errorf("associated local script is unavailable: %w", err)
		}
		ctx, cancel := s.operationContext(30 * time.Second)
		_, validationErr := s.validateLocalScriptCandidate(ctx, detail.Summary, script)
		cancel()
		if validationErr != nil {
			return subscription.State{}, fmt.Errorf("local script output is invalid: %w", validationErr)
		}
	}
	_, err = s.subscriptions.SetLocalScript(id, localScriptID)
	if err != nil {
		return subscription.State{}, err
	}
	_ = s.reconcileSelectedWithTimeoutLocked(reconcileTriggerLocalScript, nil, reconcileAutomatic)
	return s.SubscriptionState()
}

func (s *Service) validateLocalScriptCandidate(ctx context.Context, summary subscription.Summary, candidate localscript.Script) (localscript.TestResult, error) {
	return s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{script: &candidate})
}

func (s *Service) subscriptionSummary(id string) (subscription.Summary, error) {
	state, err := s.subscriptions.State()
	if err != nil {
		return subscription.Summary{}, err
	}
	for _, item := range state.Subscriptions {
		if item.ID == id {
			return item, nil
		}
	}
	return subscription.Summary{}, fmt.Errorf("subscription does not exist")
}
