package coresnapshot

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotsCopyOnlyActiveUIAndReusePersistentOriginal(t *testing.T) {
	source := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		full := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	selected := "external-ui/zashboard/v3.26.0/dist/index.html"
	old := "external-ui/zashboard/v3.25.0/dist/index.html"
	write(selected, "selected UI")
	write(old, "old UI")
	write("generations/one/bootstrap.yaml", testController+"external-ui: external-ui/zashboard/v3.26.0/dist\n")
	for _, initial := range []bool{true, true, false} {
		var archive bytes.Buffer
		if err := WriteSnapshot(source, "generations/one/bootstrap.yaml", initial, &archive); err != nil {
			t.Fatal(err)
		}
		// Each initial transfer represents a fresh protected helper session.
		destination := t.TempDir()
		if err := Receive(destination, source, &archive, map[string]string{}); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(filepath.Join(destination, filepath.FromSlash(selected)))
		if initial && err != nil {
			t.Fatal("fresh session would re-download UI", err)
		}
		if !initial && !os.IsNotExist(err) {
			t.Fatal("hot reload copied UI resources")
		}
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(old))); !os.IsNotExist(err) {
			t.Fatal("inactive version leaked into session")
		}
	}
	write("generations/one/bootstrap.yaml", testController)
	var disabled bytes.Buffer
	if err := WriteSnapshot(source, "generations/one/bootstrap.yaml", true, &disabled); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := Receive(destination, source, &disabled, map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "external-ui")); !os.IsNotExist(err) {
		t.Fatal("disabled UI was copied")
	}
	if _, err := os.Stat(filepath.Join(source, filepath.FromSlash(selected))); err != nil {
		t.Fatal("persistent resource was lost")
	}
}
