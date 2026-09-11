package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestProtectedRuleCacheReusedOnlyByItsOwner(t *testing.T) {
	root := t.TempDir()
	modified := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	for run, uid := range []uint32{501, 501, 502} {
		workspace := t.TempDir()
		directory := filepath.Join(workspace, "generations", "one")
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"bootstrap.yaml", "config.yaml"} {
			if err := os.WriteFile(filepath.Join(directory, file), []byte("rule-providers:\n  Test:\n    type: http\n    url: https://example.test/rules\n    behavior: domain\n    interval: 600\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		cache, err := newRuleCache(root, workspace, uid, "generations/one/bootstrap.yaml")
		if err != nil {
			t.Fatal(err)
		}
		var location string
		for _, name := range []string{"bootstrap.yaml", "config.yaml"} {
			data, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Providers map[string]struct{ Path string } `yaml:"rule-providers"`
			}
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			current := config.Providers["Test"].Path
			if current == "" || (location != "" && location != current) {
				t.Fatal("TUN bootstrap and final configuration do not share rule cache")
			}
			location = current
		}
		provider := filepath.Join(workspace, filepath.FromSlash(location))
		if run == 1 {
			content, err := os.ReadFile(provider)
			if err != nil || string(content) != "payload: [test.example]\n" {
				t.Fatalf("protected cache not restored: %q, %v", content, err)
			}
			info, err := os.Stat(provider)
			if err != nil || !info.ModTime().Equal(modified) {
				t.Fatal("restoring cache reset the provider age")
			}
		} else if _, err := os.Stat(provider); !os.IsNotExist(err) {
			t.Fatal("cache reused without a previous session from this UID")
		}
		if run == 0 {
			if err := os.WriteFile(provider, []byte("payload: [test.example]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(provider, modified, modified); err != nil {
				t.Fatal(err)
			}
			if err := cache.Save(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
	}
}
