package subscription

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jeemi/internal/platform/paths"
)

const fixedSubscriptionID = "abcdef0123456789abcdef0123456789"

func TestStorePreservesRawRevisionsAndRedactsListSource(t *testing.T) {
	store, setNow := newTestStore(t)
	first := []byte("port: 7890\nproxies: []\n")
	created, err := store.Create(createInput{
		Name:         "远程订阅",
		Description:  "测试",
		SourceKind:   SourceURL,
		ImportMethod: ImportURL,
		SourceURL:    "https://alice:password@example.com/private/path?token=top-secret#ignored",
		Format:       FormatYAML,
		Contents:     first,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.SourceLabel != "https://example.com" {
		t.Fatalf("SourceLabel = %q, want redacted origin", created.SourceLabel)
	}
	if strings.Contains(created.SourceLabel, "password") || strings.Contains(created.SourceLabel, "token") {
		t.Fatalf("source label leaked credentials: %q", created.SourceLabel)
	}
	text, err := store.Text(created.ID)
	if err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if text.Contents != string(first) {
		t.Fatalf("raw source changed:\n%s", text.Contents)
	}
	firstRevisionPath := filepath.Join(store.Directory(), created.ID, "revisions", created.CurrentRevisionID, "source.yaml")

	setNow(time.Date(2026, 9, 2, 11, 0, 0, 0, time.UTC))
	second := []byte("port: 7891\nproxies: []\n")
	updated, err := store.UpdateFromURL(UpdateInput{
		ID:          created.ID,
		Name:        created.Name,
		Description: created.Description,
		SourceURL:   created.SourceURL,
	}, FetchedContent{Format: FormatYAML, Contents: second})
	if err != nil {
		t.Fatalf("UpdateFromURL() error = %v", err)
	}
	if updated.RevisionCount != 2 || updated.CurrentRevisionID == created.CurrentRevisionID {
		t.Fatalf("revision was not advanced: %+v", updated)
	}
	preserved, err := os.ReadFile(firstRevisionPath)
	if err != nil {
		t.Fatalf("read first immutable revision: %v", err)
	}
	if string(preserved) != string(first) {
		t.Fatalf("first revision was modified:\n%s", preserved)
	}
	text, err = store.Text(created.ID)
	if err != nil || text.Contents != string(second) {
		t.Fatalf("current text = %q, error = %v", text.Contents, err)
	}
	state, err := store.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(state.Subscriptions) != 0 {
		t.Fatalf("deleted subscription is still listed: %+v", state.Subscriptions)
	}
	if _, err := os.Stat(filepath.Join(store.Directory(), created.ID)); !os.IsNotExist(err) {
		t.Fatalf("deleted subscription directory still exists: %v", err)
	}
}

func TestStoreReusesIdenticalRevisionAndPersistsAssociation(t *testing.T) {
	store, setNow := newTestStore(t)
	contents := []byte("mode: rule\n")
	created, err := store.Create(createInput{
		Name:         "Same",
		SourceKind:   SourceURL,
		ImportMethod: ImportURL,
		SourceURL:    "https://example.com/sub",
		Format:       FormatYAML,
		Contents:     contents,
	})
	if err != nil {
		t.Fatal(err)
	}
	setNow(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))
	updated, err := store.UpdateFromURL(UpdateInput{
		ID: created.ID, Name: created.Name, SourceURL: created.SourceURL,
	}, FetchedContent{Format: FormatYAML, Contents: contents, RemoteProfile: RemoteProfile{
		HasTraffic: true, UploadBytes: 10, DownloadBytes: 20, TotalBytes: 100,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RevisionCount != 1 || updated.LastFetchedAt == created.LastFetchedAt {
		t.Fatalf("identical refresh state is wrong: %+v", updated)
	}
	if !updated.RemoteProfile.HasTraffic || updated.RemoteProfile.TotalBytes != 100 || updated.RemoteProfile.ObservedAt == "" {
		t.Fatalf("identical body did not update response metadata: %+v", updated.RemoteProfile)
	}
	localConfigID := "0123456789abcdef0123456789abcdef"
	associated, err := store.SetLocalConfig(created.ID, localConfigID)
	if err != nil {
		t.Fatal(err)
	}
	if associated.LocalConfigID != localConfigID {
		t.Fatalf("LocalConfigID = %q", associated.LocalConfigID)
	}
	references, err := store.ReferencingLocalConfig(localConfigID)
	if err != nil || len(references) != 1 || references[0].ID != created.ID {
		t.Fatalf("references = %+v, error = %v", references, err)
	}
}

func TestStorePersistsIconsAndKeepsLocalHandlersMutuallyExclusive(t *testing.T) {
	store, _ := newTestStore(t)
	created, err := store.Create(createInput{
		Name: "Icon", SourceKind: SourceURL, ImportMethod: ImportURL,
		SourceURL: "https://example.com/sub", Format: FormatYAML,
		Contents: []byte("mode: rule\n"), IconKind: IconEmoji, Icon: "🐱",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.IconKind != IconEmoji || created.Icon != "🐱" {
		t.Fatalf("subscription icon was not persisted: %+v", created.Summary)
	}

	localConfigID := "0123456789abcdef0123456789abcdef"
	localScriptID := "fedcba9876543210fedcba9876543210"
	if _, err := store.SetLocalConfig(created.ID, localConfigID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLocalScript(created.ID, localScriptID); err == nil {
		t.Fatal("SetLocalScript() accepted a subscription that still has a local configuration")
	}
	if _, err := store.SetLocalConfig(created.ID, ""); err != nil {
		t.Fatal(err)
	}
	associated, err := store.SetLocalScript(created.ID, localScriptID)
	if err != nil {
		t.Fatal(err)
	}
	if associated.LocalConfigID != "" || associated.LocalScriptID != localScriptID {
		t.Fatalf("unexpected handler association: %+v", associated.Summary)
	}
	if _, err := store.SetLocalConfig(created.ID, localConfigID); err == nil {
		t.Fatal("SetLocalConfig() accepted a subscription that still has a local script")
	}
	references, err := store.ReferencingLocalScript(localScriptID)
	if err != nil || len(references) != 1 || references[0].ID != created.ID {
		t.Fatalf("ReferencingLocalScript() = %+v, error = %v", references, err)
	}
}

func TestStorePersistsRuleProviderSwitchesWithoutChangingSourceRevision(t *testing.T) {
	store, _ := newTestStore(t)
	created, err := store.Create(createInput{
		Name: "Providers", SourceKind: SourceFile, ImportMethod: ImportFile,
		OriginalFileName: "providers.yaml", Format: FormatYAML,
		Contents: []byte("rule-providers:\n  ads: {type: http}\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := store.SetRuleProviderEnabled(created.ID, "ads", false)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.RuleProviderOverrideRevision != 1 || len(disabled.DisabledRuleProviders) != 1 || disabled.DisabledRuleProviders[0] != "ads" {
		t.Fatalf("unexpected disabled-provider state: %+v", disabled.Summary)
	}
	if disabled.CurrentRevisionID != created.CurrentRevisionID || disabled.RevisionCount != created.RevisionCount {
		t.Fatal("provider switch mutated the immutable subscription revision")
	}
	unchanged, err := store.SetRuleProviderEnabled(created.ID, "ads", false)
	if err != nil || unchanged.RuleProviderOverrideRevision != 1 {
		t.Fatalf("idempotent disable changed revision: %+v err=%v", unchanged.Summary, err)
	}
	state, err := store.State()
	if err != nil || len(state.Subscriptions) != 1 || len(state.Subscriptions[0].DisabledRuleProviders) != 1 {
		t.Fatalf("disabled provider did not survive reload: %+v err=%v", state, err)
	}
	enabled, err := store.SetRuleProviderEnabled(created.ID, "ads", true)
	if err != nil || enabled.RuleProviderOverrideRevision != 2 || len(enabled.DisabledRuleProviders) != 0 {
		t.Fatalf("unexpected re-enabled state: %+v err=%v", enabled.Summary, err)
	}
}

func TestStoreRejectsUnsafeOrUnsupportedDocuments(t *testing.T) {
	store, _ := newTestStore(t)
	for _, testCase := range []struct {
		name     string
		format   Format
		contents []byte
	}{
		{name: "multiple YAML documents", format: FormatYAML, contents: []byte("a: 1\n---\nb: 2\n")},
		{name: "JSON array root", format: FormatJSON, contents: []byte(`[1, 2]`)},
		{name: "invalid JSON", format: FormatJSON, contents: []byte(`{broken`)},
		{name: "unsupported format", format: Format("ini"), contents: []byte("a=1")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := store.Create(createInput{
				Name: "Invalid", SourceKind: SourceFile, ImportMethod: ImportFile,
				OriginalFileName: "invalid.txt", Format: testCase.format, Contents: testCase.contents,
			})
			if err == nil {
				t.Fatal("Create() accepted an unsafe or unsupported document")
			}
		})
	}
}

func TestStoreFileMetadataPersistsOnlyTheOriginalBaseName(t *testing.T) {
	store, _ := newTestStore(t)
	created, err := store.Create(createInput{
		Name: "Imported", SourceKind: SourceFile, ImportMethod: ImportFile,
		OriginalFileName: "subscription.txt", Format: FormatText, Contents: []byte("mode: rule\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(filepath.Join(store.Directory(), created.ID, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), `"originalFileName": "subscription.txt"`) || !strings.Contains(string(metadata), `"sourceUrl": ""`) {
		t.Fatalf("unexpected file metadata:\n%s", metadata)
	}
}

func newTestStore(t *testing.T) (*Store, func(time.Time)) {
	t.Helper()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	store, err := NewStore(StoreOptions{
		DataDirectory: filepath.Join(t.TempDir(), paths.DataDirectoryName),
		Now:           func() time.Time { return now },
		NewID:         func() (string, error) { return fixedSubscriptionID, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, func(next time.Time) { now = next }
}
