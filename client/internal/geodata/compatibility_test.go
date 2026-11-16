package geodata

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGEOManifestPreservesUnknownFieldsOnSave(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "geodata")
	os.MkdirAll(directory, 0700)
	path := filepath.Join(directory, "manifest.json")
	raw := []byte(`{"version":1,"futureMetadata":{"keep":9007199254740993}}`)
	os.WriteFile(path, raw, 0600)
	manager := &Manager{directory: directory}
	document, err := manager.loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if !bytes.Equal(raw, before) {
		t.Fatal("read rewrote GEO manifest")
	}
	document.LastCheckedAt = "2026-11-16T00:00:00Z"
	if err := manager.saveManifest(document); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Contains(after, []byte("9007199254740993")) {
		t.Fatal("GEO save lost extension")
	}
}

func TestWorkspaceSwitchReactivatesItsOwnGEORevision(t *testing.T) {
	shared := t.TempDir()
	prefs := DefaultPreferences()
	var managers []*Manager
	for _, content := range []string{"workspace-one-database", "workspace-two-database"} {
		m, err := NewManager(ManagerOptions{DataDirectory: t.TempDir(), RuntimeDataDirectory: shared, Validator: &acceptingValidator{}})
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "geoip.metadb")
		os.WriteFile(file, []byte(content), 0600)
		if _, err := m.Import(context.Background(), KindGeoIPMMDB, file, "mihomo", "v1", prefs); err != nil {
			t.Fatal(err)
		}
		activation, err := m.PrepareActivation(prefs)
		if err != nil {
			t.Fatal(err)
		}
		if err := activation.Commit(); err != nil {
			t.Fatal(err)
		}
		managers = append(managers, m)
	}
	for i := 0; i < 4; i++ {
		m := managers[i%2]
		pending, err := m.HasPending(prefs)
		if err != nil || !pending {
			t.Fatal("other workspace's active file was mistaken for this revision", err)
		}
		activation, err := m.PrepareActivation(prefs)
		if err != nil {
			t.Fatal(err)
		}
		if err := activation.Commit(); err != nil {
			t.Fatal(err)
		}
		pending, err = m.HasPending(prefs)
		if err != nil || pending {
			t.Fatal("activation did not restore this workspace", err)
		}
		manifest, err := m.loadManifest()
		if err != nil {
			t.Fatal(err)
		}
		if !fileMatchesSHA256(filepath.Join(shared, "runtime", "mihomo", "geoip.metadb"), manifest.Desired[string(KindGeoIPMMDB)]) {
			t.Fatal("wrong active GEO bytes")
		}
	}
}
