package subscription

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/config/fallbackoverride"
)

func TestFirstReleaseFallbackIsExplicitAndPreservesSource(t *testing.T) {
	store, _ := newTestStore(t)
	source := []byte("rules: ['MATCH,DIRECT']\n")
	created, err := store.Create(createInput{Name: "Fixture", SourceKind: SourceURL, ImportMethod: ImportURL, SourceURL: "https://example.test/sub", Format: FormatYAML, Contents: source})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Directory(), created.ID, "metadata.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var metadata metadataDocument
	if err := json.Unmarshal(before, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.ManifestVersion != 1 || metadata.Fallback.Mode != fallbackoverride.ModeNone {
		t.Fatal("new subscription lacks its initial fallback selection")
	}
	unchanged, err := store.SetFallback(created.ID, created.Fallback, 0, false)
	if err != nil || unchanged.FallbackOverrideRevision != 0 {
		t.Fatal("unchanged preference created a revision", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("unchanged preference rewrote metadata", err)
	}
	changed, err := store.SetFallback(created.ID, fallbackoverride.Selection{Mode: fallbackoverride.ModeDirect}, 0, false)
	if err != nil || changed.FallbackOverrideRevision != 1 {
		t.Fatal("fallback was not persisted", err)
	}
	if _, err := store.SetFallback(created.ID, created.Fallback, 0, false); err == nil {
		t.Fatal("stale fallback revision was accepted")
	}
	text, err := store.Text(created.ID)
	if err != nil || text.Contents != string(source) || changed.CurrentRevisionID != created.CurrentRevisionID {
		t.Fatal("fallback change modified the subscription source", err)
	}
}
