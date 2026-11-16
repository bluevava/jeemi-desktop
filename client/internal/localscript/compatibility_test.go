package localscript

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScriptManifestKeepsUnknownFieldsWhenSourceChanges(t *testing.T) {
	store, err := NewStore(StoreOptions{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Save(SaveInput{Name: "Script", Contents: "function main(config) { return config; }"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.root, created.ID, "manifest.json")
	raw, _ := os.ReadFile(path)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["futureManifest"] = json.RawMessage(`{"keep":9007199254740993}`)
	raw, _ = json.Marshal(fields)
	os.WriteFile(path, raw, 0600)
	loaded, err := store.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(SaveInput{ID: loaded.ID, Name: "changed", Contents: loaded.Contents + "\n// edit\n"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Contains(after, []byte("9007199254740993")) {
		t.Fatal("script edit lost newer manifest fields")
	}
}
