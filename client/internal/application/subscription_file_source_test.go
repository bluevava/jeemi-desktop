package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"jeemi/internal/localscript"
	mihomoruntime "jeemi/internal/mihomo/runtime"
	"jeemi/internal/subscription"
)

func fileSubscriptionService(t *testing.T) (*Service, string, string) {
	t.Helper()
	s, err := newTestService(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	useDomainOnlyFakeIPFilter(t, s)
	path := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(path, []byte(fallbackFixture), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.ImportSubscriptionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, state.SelectedSubscriptionID, path
}

func TestFileRefreshValidatesAndPreservesLocalHandlerOnFailure(t *testing.T) {
	s, id, path := fileSubscriptionService(t)
	script, err := s.SaveLocalScript(localscript.SaveInput{Name: "Validate source", Contents: "function main(c) { if(c.fail) throw new Error('rejected'); return c; }"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetSubscriptionLocalScript(id, script.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Subscription(id)
	resolvedBefore, _ := s.RuntimeConfigurationText()
	candidate := fallbackFixture + "fail: true\n"
	if err := os.WriteFile(path, []byte(candidate), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := s.CheckSubscriptionRefresh(id)
	if err != nil || result.Conflict == nil {
		t.Fatal("file refresh bypassed local handler validation", err)
	}
	if _, err := s.ResolveSubscriptionRefresh(result.Conflict.Token, false); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Subscription(id)
	view, _ := s.RuntimeConfigurationText()
	if !reflect.DeepEqual(before, after) || view.Contents != resolvedBefore.Contents {
		t.Fatal("cancelled refresh changed the saved source or runtime")
	}
	result, err = s.CheckSubscriptionRefresh(id)
	if err != nil || result.Conflict == nil {
		t.Fatal(err)
	}
	// Confirmation commits the already checked bytes, even if the source changes.
	if err := os.WriteFile(path, []byte("rules: ['MATCH,Missing']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveSubscriptionRefresh(result.Conflict.Token, true); err != nil {
		t.Fatal(err)
	}
	text, _ := s.SubscriptionText(id)
	after, _ = s.Subscription(id)
	if text.Contents != candidate || after.LocalScriptID != "" || after.SourceFilePath != path {
		t.Fatal("confirmation reread the source or lost its path")
	}
	if _, err := s.RefreshSubscription(id); err == nil {
		t.Fatal("invalid final routing configuration was saved")
	}
	text, _ = s.SubscriptionText(id)
	if text.Contents != candidate || s.runtimeManager.Status().State != mihomoruntime.StateStopped {
		t.Fatal("failed refresh changed the cache or started the core")
	}
}

func TestNativeFileSelectionIsADraftAndSaveIsTransactional(t *testing.T) {
	s, id, oldPath := fileSubscriptionService(t)
	before, _ := s.Subscription(id)
	newPath := filepath.Join(t.TempDir(), "replacement.yaml")
	if err := os.WriteFile(newPath, []byte("rules: ['MATCH,Missing']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	selection, err := s.StageSubscriptionFile(id, newPath)
	if err != nil || selection.FileName != "replacement.yaml" || selection.Token == "" {
		t.Fatal("native file choice was not staged", err)
	}
	encoded, _ := json.Marshal(selection)
	if strings.Contains(string(encoded), "sourceFilePath") || strings.Contains(string(encoded), filepath.ToSlash(filepath.Dir(newPath))) {
		t.Fatal("picker leaked the path")
	}
	afterChoice, _ := s.Subscription(id)
	if !reflect.DeepEqual(before, afterChoice) {
		t.Fatal("choosing a file changed saved metadata")
	}
	input := subscription.UpdateInput{ID: id, Name: "Edited", SourceFileToken: selection.Token}
	if _, err := s.UpdateSubscription(input); err == nil {
		t.Fatal("invalid replacement configuration was saved")
	}
	afterFailure, _ := s.Subscription(id)
	if !reflect.DeepEqual(before, afterFailure) {
		t.Fatal("failed replacement changed source or name")
	}
	contents := fallbackFixture + "newFile: true\n"
	if err := os.WriteFile(newPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := s.UpdateSubscription(input)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := s.Subscription(id)
	text, _ := s.SubscriptionText(id)
	if after.SourceFilePath != newPath || after.SourceLabel != "replacement.yaml" || after.Name != "Edited" || text.Contents != contents || state.SelectedSubscriptionID != id || state.Projection.Status != projectionReady {
		t.Fatal("replacement did not update the shared runtime flow")
	}
	if _, err := s.UpdateSubscription(input); err == nil {
		t.Fatal("consumed file token was reused")
	}
	oldContents, _ := os.ReadFile(oldPath)
	if string(oldContents) != fallbackFixture {
		t.Fatal("replacement modified the original external file")
	}
	if err := os.Remove(newPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RefreshSubscription(id); err == nil {
		t.Fatal("removed source unexpectedly refreshed")
	}
	afterReadFailure, _ := s.Subscription(id)
	if !reflect.DeepEqual(after, afterReadFailure) {
		t.Fatal("read failure changed the successful replacement")
	}
	// A missing external file does not prevent ordinary metadata edits.
	if _, err := s.UpdateSubscription(subscription.UpdateInput{ID: id, Name: "Renamed offline"}); err != nil {
		t.Fatal(err)
	}
}

func TestFilePickerTokensRejectStaleExpiredAndOtherSubscriptionUse(t *testing.T) {
	for _, scenario := range []string{"expired", "stale", "wrong-id", "unknown-token"} {
		t.Run(scenario, func(t *testing.T) {
			s, id, path := fileSubscriptionService(t)
			selection, err := s.StageSubscriptionFile(id, path)
			if err != nil {
				t.Fatal(err)
			}
			input := subscription.UpdateInput{ID: id, Name: "Edited", SourceFileToken: selection.Token}
			switch scenario {
			case "expired":
				s.pendingFileSelection.expires = time.Now().Add(-time.Second)
			case "stale":
				if _, err := s.UpdateSubscription(subscription.UpdateInput{ID: id, Name: "Other edit"}); err != nil {
					t.Fatal(err)
				}
			case "wrong-id":
				input.ID = strings.Repeat("c", 32)
			case "unknown-token":
				input.SourceFileToken = "not-a-file-path"
			}
			before, _ := s.Subscription(id)
			if _, err := s.UpdateSubscription(input); err == nil || !strings.Contains(err.Error(), "subscription_file_selection_expired") {
				t.Fatal("invalid picker token was accepted", err)
			}
			after, _ := s.Subscription(id)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("invalid token changed the subscription")
			}
		})
	}
}
