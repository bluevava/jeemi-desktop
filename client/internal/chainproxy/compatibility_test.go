package chainproxy

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestChainLibraryPreservesUnknownFieldsOnCommit(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.Commit(0, Library{Version: Version, Groups: []Group{{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "group", Kind: "manual"}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(store.file)
	var fields map[string]any
	json.Unmarshal(raw, &fields)
	fields["futureLibrary"] = "keep"
	fields["groups"].([]any)[0].(map[string]any)["futureGroup"] = "keep"
	raw, _ = json.Marshal(fields)
	os.WriteFile(store.file, raw, 0600)
	for i := 0; i < 3; i++ {
		next, err = store.Load()
		if err != nil {
			t.Fatal(err)
		}
		next.Groups[0].Name = "edited"
		if _, err := store.Commit(next.Revision, next); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(store.file)
		if !bytes.Contains(after, []byte("futureLibrary")) || !bytes.Contains(after, []byte("futureGroup")) {
			t.Fatal("chain edit dropped extensions")
		}
	}
}
