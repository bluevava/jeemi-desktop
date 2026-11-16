package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	"jeemi/internal/subscription"
)

type pendingSubscriptionFileSelection struct {
	token   string
	path    string
	expires time.Time
	before  subscription.Detail
}

// StageSubscriptionFile is called only with a native dialog result. Selecting
// a file changes no saved metadata; Save reads and validates its current bytes.
func (s *Service) StageSubscriptionFile(id, path string) (subscription.FileSelectionResult, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	before, err := s.subscriptions.Get(id)
	if err != nil {
		return subscription.FileSelectionResult{}, err
	}
	if before.SourceKind != subscription.SourceFile {
		return subscription.FileSelectionResult{}, fmt.Errorf("subscription file selection requires a file source")
	}
	path, err = subscription.NormalizeFileSourcePath(path)
	if err != nil {
		return subscription.FileSelectionResult{}, err
	}
	var random [24]byte
	if _, err := rand.Read(random[:]); err != nil {
		return subscription.FileSelectionResult{}, fmt.Errorf("create subscription file selection")
	}
	token := hex.EncodeToString(random[:])
	s.pendingFileSelection = &pendingSubscriptionFileSelection{token: token, path: path, before: before, expires: time.Now().Add(time.Hour)}
	return subscription.FileSelectionResult{Token: token, FileName: filepath.Base(path)}, nil
}

// Caller holds reconcileMu throughout the read, candidate validation and commit.
func (s *Service) updateSubscriptionSource(ctx context.Context, input subscription.UpdateInput) error {
	validate := func(summary subscription.Summary, source []byte) error {
		_, err := s.validateSubscriptionCandidate(ctx, summary, compositionCandidate{source: source})
		return err
	}
	if input.SourceFileToken == "" {
		_, err := s.subscriptions.UpdateValidated(ctx, input, validate)
		return err
	}
	selection := s.pendingFileSelection
	if selection == nil || selection.token != input.SourceFileToken || selection.before.ID != input.ID || time.Now().After(selection.expires) {
		return fmt.Errorf("subscription_file_selection_expired")
	}
	current, err := s.subscriptions.Get(input.ID)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, selection.before) {
		return fmt.Errorf("subscription_file_selection_expired")
	}
	input.SourceFileToken = ""
	if _, err := s.subscriptions.UpdateFileValidated(ctx, input, selection.path, validate); err != nil {
		return err // Keep the picker draft available for a corrected file/retry.
	}
	s.pendingFileSelection = nil
	return nil
}
