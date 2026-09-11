package localscript

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"jeemi/internal/platform/paths"
)

const fixedScriptID = "0123456789abcdef0123456789abcdef"

func TestStoreCreatesUpdatesAndDeletesScript(t *testing.T) {
	now := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	store, err := NewStore(StoreOptions{
		DataDirectory: filepath.Join(t.TempDir(), paths.DataDirectoryName),
		Now:           func() time.Time { return now },
		NewID:         func() (string, error) { return fixedScriptID, nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := store.Save(SaveInput{
		Name: "Rewrite", Description: "Adjust one field",
		Contents: "const main = (config) => {\n  return config;\n};\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != fixedScriptID || created.Revision != 1 || created.LineCount != 3 || created.SizeBytes <= 0 {
		t.Fatalf("unexpected created script: %+v", created)
	}

	now = now.Add(time.Hour)
	updated, err := store.Save(SaveInput{
		ID: created.ID, Name: "Rewrite v2", Description: created.Description,
		Contents: "function main(config) {\n  config.mode = 'direct';\n  return config;\n}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.CreatedAt != created.CreatedAt || updated.UpdatedAt == created.UpdatedAt || updated.LineCount != 4 {
		t.Fatalf("unexpected updated script: %+v", updated)
	}

	state, err := store.State()
	if err != nil || len(state.Scripts) != 1 || state.Scripts[0].Name != "Rewrite v2" {
		t.Fatalf("State() = %+v, error = %v", state, err)
	}
	state, err = store.Delete(created.ID)
	if err != nil || len(state.Scripts) != 0 {
		t.Fatalf("Delete() = %+v, error = %v", state, err)
	}
}

func TestRemoteScriptMetadataAndReadOnlyCache(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: filepath.Join(t.TempDir(), paths.DataDirectoryName)})
	if err != nil {
		t.Fatal(err)
	}
	input := SaveInput{Name: "Remote", SourceURL: "https://example.test/script?token=private", Contents: "function main(c) {return c;}"}
	script, err := store.Save(input)
	if err != nil || script.SourceType != "url" || script.SourceURL != input.SourceURL {
		t.Fatal("source metadata not saved", err)
	}
	manifest := filepath.Join(store.Directory(), script.ID, "manifest.json")
	before, _ := os.ReadFile(manifest)
	state, err := store.State()
	if err != nil || state.Scripts[0].SourceType != "url" {
		t.Fatal("URL kind missing from summary", err)
	}
	loaded, err := store.Get(script.ID)
	after, _ := os.ReadFile(manifest)
	if err != nil || !reflect.DeepEqual(script, loaded) || string(before) != string(after) {
		t.Fatal("cache read changed persisted state", err)
	}
	input.ID, input.ExpectedRevision = script.ID, script.Revision
	input.SourceURL = "file:///invalid"
	if _, err := store.Save(input); err == nil {
		t.Fatal("invalid source metadata accepted")
	}
	input.SourceURL = ""
	text, err := store.Save(input)
	if err != nil || text.SourceType != "text" || text.SourceURL != "" {
		t.Fatal("URL script cannot become text", err)
	}
	if _, err := store.Save(input); err == nil {
		t.Fatal("stale revision replaced newer script")
	}
}
