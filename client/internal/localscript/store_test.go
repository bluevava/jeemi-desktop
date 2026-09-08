package localscript

import (
	"path/filepath"
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
