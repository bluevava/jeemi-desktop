package runtime

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDurableSelectionsUseConfigurationRootAndKeepExtensions(t *testing.T) {
	root, configuration := t.TempDir(), t.TempDir()
	manager, err := NewManager(Options{DataDirectory: root, ConfigurationDirectory: configuration})
	if err != nil {
		t.Fatal(err)
	}
	if manager.store.root != filepath.Join(root, "runtime", "mihomo") || manager.selections.root != filepath.Join(configuration, "state", "mihomo", "proxy-selections") {
		t.Fatal("configuration and session roots were not separated")
	}
	id := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path, _ := manager.selections.path(id)
	os.MkdirAll(filepath.Dir(path), 0700)
	raw := []byte(`{"version":1,"subscriptionId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","selections":{"group":"a"},"future":{"keep":true}}`)
	os.WriteFile(path, raw, 0600)
	if err := manager.selections.merge(id, map[string]string{"group": "b"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Contains(after, []byte(`"future"`)) {
		t.Fatal("selection write lost extension")
	}
}
