package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func writeSourceFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFileRefreshPersistsSourceAndReusesImmutableRevisions(t *testing.T) {
	store, setNow := newTestStore(t)
	fetcher := &queuedFetcher{}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "原始 订阅.yaml")
	first := "# original\nmode: rule\n"
	writeSourceFile(t, path, first)
	state, err := manager.ImportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	id := state.Subscriptions[0].ID
	before, _ := manager.Get(id)
	if before.SourceFilePath != path || before.SourceLabel != filepath.Base(path) || before.AutoUpdateIntervalMinutes != 0 {
		t.Fatal("file source identity was not saved")
	}
	for _, exposed := range []any{state, before} {
		encoded, _ := json.Marshal(exposed)
		if strings.Contains(string(encoded), "sourceFilePath") || strings.Contains(string(encoded), filepath.ToSlash(filepath.Dir(path))) {
			t.Fatal("absolute source path crossed the UI boundary")
		}
	}
	metadataPath := filepath.Join(store.Directory(), id, "metadata.json")
	metadataBytes, _ := os.ReadFile(metadataPath)
	var metadata metadataDocument
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil || metadata.SourceFilePath != path {
		t.Fatal("source path is not persistent", err)
	}
	// Reopen the manager to prove this is not just a remembered dialog path.
	manager, _ = NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
	if _, err := manager.SetLocalConfig(id, strings.Repeat("b", 32)); err != nil {
		t.Fatal(err)
	}
	second := "# updated\nmode: global\n"
	writeSourceFile(t, path, second)
	setNow(time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC))
	if _, err := manager.Refresh(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	after, _ := manager.Get(id)
	if after.ID != before.ID || after.Name != before.Name || after.CreatedAt != before.CreatedAt || after.RevisionCount != 2 || after.CurrentRevisionID == before.CurrentRevisionID || after.LocalConfigID != strings.Repeat("b", 32) {
		t.Fatal("refresh lost subscription identity or associations")
	}
	text, _ := manager.Text(id)
	old, err := os.ReadFile(filepath.Join(store.Directory(), id, "revisions", before.CurrentRevisionID, "source.yaml"))
	if err != nil || text.Contents != second || string(old) != first {
		t.Fatal("refresh did not preserve immutable source revisions", err)
	}
	setNow(time.Date(2026, 11, 12, 13, 0, 0, 0, time.UTC))
	if _, err := manager.Refresh(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	repeated, _ := manager.Get(id)
	if repeated.RevisionCount != 2 || repeated.CurrentRevisionID != after.CurrentRevisionID || repeated.LastFetchedAt == after.LastFetchedAt {
		t.Fatal("same bytes did not reuse the revision and update refresh time")
	}
	if len(fetcher.urls) != 0 {
		t.Fatal("file refresh used the network")
	}
}

func TestFileRefreshFailuresPreserveMetadataAndCachedSource(t *testing.T) {
	for _, failure := range []string{"missing", "directory", "empty", "oversize", "syntax", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			store, _ := newTestStore(t)
			manager, _ := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}})
			path := filepath.Join(t.TempDir(), "private-source.yaml")
			writeSourceFile(t, path, "mode: rule\n")
			state, err := manager.ImportFile(path)
			if err != nil {
				t.Fatal(err)
			}
			id := state.Subscriptions[0].ID
			metadataPath := filepath.Join(store.Directory(), id, "metadata.json")
			before, _ := os.ReadFile(metadataPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch failure {
			case "missing", "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if failure == "directory" {
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
			case "empty":
				writeSourceFile(t, path, "")
			case "oversize":
				writeSourceFile(t, path, strings.Repeat("x", maxSubscriptionBytes+1))
			case "syntax":
				writeSourceFile(t, path, "proxies: [\n")
			case "cancelled":
				cancel()
			}
			if _, err := manager.Refresh(ctx, id); err == nil || strings.Contains(err.Error(), path) {
				t.Fatal("failed read was accepted or leaked the source path")
			}
			after, _ := os.ReadFile(metadataPath)
			text, textErr := manager.Text(id)
			if string(before) != string(after) || textErr != nil || text.Contents != "mode: rule\n" {
				t.Fatal("failed refresh changed metadata or cached source", textErr)
			}
			// Listing and startup checks must not require the external source.
			if _, err := manager.State(); err != nil {
				t.Fatal(err)
			}
			if err := CheckStartupConfiguration(filepath.Dir(store.Directory())); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFileSourceReselectionIsValidatedBeforeAtomicCommit(t *testing.T) {
	store, _ := newTestStore(t)
	manager, _ := NewManager(ManagerOptions{Store: store, Fetcher: &queuedFetcher{}})
	// Older imports without a path remain readable and editable, without guessing
	// that the managed cached revision is the original external source.
	before, err := store.Create(createInput{Name: "Existing", SourceKind: SourceFile, ImportMethod: ImportFile, OriginalFileName: "old.yaml", Format: FormatYAML, Contents: []byte("mode: rule\n")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Refresh(context.Background(), before.ID); err == nil || err.Error() != "subscription_file_source_missing" {
		t.Fatal("missing original path was not distinguished", err)
	}
	path := filepath.Join(t.TempDir(), "new.json")
	writeSourceFile(t, path, `{"mode":"global"}`)
	input := UpdateInput{ID: before.ID, Name: "Renamed", Description: "New description"}
	if _, err := manager.UpdateFileValidated(context.Background(), input, path, func(Summary, []byte) error { return fmt.Errorf("candidate rejected") }); err == nil {
		t.Fatal("invalid final configuration was saved")
	}
	afterFailure, _ := manager.Get(before.ID)
	if !reflect.DeepEqual(before, afterFailure) {
		t.Fatal("reselection failure changed path or metadata")
	}
	if _, err := manager.UpdateFileValidated(context.Background(), input, path, func(_ Summary, contents []byte) error {
		if string(contents) != `{"mode":"global"}` {
			t.Fatal("validator received old source")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := manager.Get(before.ID)
	if after.SourceFilePath != path || after.SourceLabel != "new.json" || after.Name != "Renamed" || after.RevisionCount != 2 || after.Format != FormatJSON {
		t.Fatal("source replacement was not committed with metadata")
	}
	writeSourceFile(t, path, `{"mode":"direct"}`)
	candidate, err := manager.PrepareRefresh(context.Background(), before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateMetadata(UpdateInput{ID: before.ID, Name: "Concurrent edit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.CommitRefresh(candidate, false); err == nil {
		t.Fatal("stale file candidate overwrote metadata")
	}
	text, _ := manager.Text(before.ID)
	if text.Contents != `{"mode":"global"}` {
		t.Fatal("stale candidate changed the current source")
	}
}
