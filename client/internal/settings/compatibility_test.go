package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/configfile"
)

func TestCompatibleSettingsRetainUnknownFieldsAndUntouchedDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := []byte(`{"mihomo":{"selectedVersion":"old","future":123},"subscriptions":{"selectorViewMode":"future-view"},"runtime":{"allowLan":false,"lanBypassRules":[],"future":{"n":9007199254740993}},"futureRoot":true}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		store, err := NewStore(path)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(path)
		loaded, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("read rewrote document")
		}
		if loaded.Runtime.AllowLAN || len(loaded.Runtime.LANBypassRules) != 0 {
			t.Fatal("explicit settings changed")
		}
		if _, err := store.SetMihomoVersion("new"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.SetRuntimePreferences(loaded.Runtime); err != nil {
			t.Fatal(err)
		}
		saved, _ := os.ReadFile(path)
		var fields map[string]json.RawMessage
		json.Unmarshal(saved, &fields)
		if string(fields["futureRoot"]) != "true" || !bytes.Contains(fields["runtime"], []byte("9007199254740993")) || !bytes.Contains(fields["subscriptions"], []byte("future-view")) || !bytes.Contains(fields["mihomo"], []byte("future")) {
			t.Fatal("unrelated edit lost newer data")
		}
	}
}

func TestUnknownRuntimeEnumNeverFallsBackOrOffersDeletion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	raw := []byte(`{"runtime":{"outboundMode":"future-policy"}}`)
	os.WriteFile(path, raw, 0600)
	store, _ := NewStore(path)
	err := configfile.Recover(context.Background(), root, func() error { _, err := store.Load(); return err }, func(*configfile.Failure) (bool, error) {
		t.Fatal("unsupported configuration offered for deletion")
		return true, nil
	})
	if !configfile.IsUnsupported(err) {
		t.Fatal("unsupported policy was replaced by a default", err)
	}
	if _, err := store.SetMihomoVersion("new"); !configfile.IsUnsupported(err) {
		t.Fatal("unsupported document was saved", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("unsupported data changed")
	}
}
