package configworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"settings.json":                            `{"legacy":true}`,
		"subscriptions/a/metadata.json":            `{"id":"a"}`,
		"subscriptions/a/revisions/source.yaml":    "proxies: []\n",
		"local-configs/resources/library.json":     `{"rules":[]}`,
		"geodata/revisions/asset":                  "geo",
		"state/mihomo/proxy-selections/a.json":     `{"choice":"a"}`,
		"core/mihomo/binary":                       "core-do-not-copy",
		"runtime/mihomo/generations/a/config.yaml": "secret-do-not-copy",
		"network-recovery.json":                    "global-do-not-copy",
	} {
		put(t, filepath.Join(root, filepath.FromSlash(name)), []byte(body))
	}
	return root
}
func put(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
func valid(context.Context, string) error { return nil }

func TestMigrationCopiesOnceAndOlderEditsDoNotOverwriteNewWorkspace(t *testing.T) {
	root := fixture(t)
	ctx := context.Background()
	legacy, _ := os.ReadFile(filepath.Join(root, "settings.json"))
	workspace, err := Prepare(ctx, root, Options{Validate: valid})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"settings.json", "subscriptions/a/metadata.json", "subscriptions/a/revisions/source.yaml", "local-configs/resources/library.json", "geodata/revisions/asset", "state/mihomo/proxy-selections/a.json"} {
		before, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		after, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("copy changed configuration", path, err)
		}
	}
	for _, path := range []string{"core", "runtime", "network-recovery.json"} {
		if _, err := os.Stat(filepath.Join(workspace, path)); !os.IsNotExist(err) {
			t.Fatal("copied global state", path)
		}
	}
	put(t, filepath.Join(workspace, "settings.json"), []byte(`{"newer":true}`))
	for i := 0; i < 5; i++ {
		put(t, filepath.Join(root, "settings.json"), []byte(fmt.Sprintf(`{"legacyEdit":%d}`, i)))
		selected, err := Prepare(ctx, root, Options{Validate: func(context.Context, string) error { t.Fatal("re-migrated committed workspace"); return nil }})
		if err != nil || selected != workspace {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(filepath.Join(workspace, "settings.json"))
		if string(after) != `{"newer":true}` {
			t.Fatal("old version overwrote new version")
		}
	}
	if string(legacy) != `{"legacy":true}` {
		t.Fatal("invalid fixture")
	}
	// Incompatible schema changes copy only the registered predecessor.
	second, err := Prepare(ctx, root, Options{Version: 2, SourceVersion: 1, Validate: valid, Migrate: func(ctx context.Context, stage string) error {
		put(t, filepath.Join(stage, "settings.json"), []byte(`{"schema2":true}`))
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		older, err := Prepare(ctx, root, Options{Validate: valid})
		if err != nil || older != workspace {
			t.Fatal(err)
		}
		put(t, filepath.Join(older, "settings.json"), []byte(`{"olderSchemaChanged":true}`))
		newer, err := Prepare(ctx, root, Options{Version: 2, SourceVersion: 1, Validate: valid})
		if err != nil || newer != second {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(filepath.Join(newer, "settings.json"))
		if string(after) != `{"schema2":true}` {
			t.Fatal("schema switch overwrote committed data")
		}
	}
}

func TestMigrationFailuresNeverPublishPartialDataOrChangeOriginal(t *testing.T) {
	for _, stage := range []string{"copy", "transform", "validate", "publish", "cancel", "source-changed"} {
		t.Run(stage, func(t *testing.T) {
			root := fixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			before, _ := os.ReadFile(filepath.Join(root, "settings.json"))
			options := Options{Validate: valid}
			sentinel := errors.New("injected disk or migration failure")
			switch stage {
			case "copy":
				count := 0
				options.copyFile = func(ctx context.Context, from, to string) (fileRecord, error) {
					count++
					if count == 2 {
						return fileRecord{}, sentinel
					}
					return copyRegularFile(ctx, from, to)
				}
			case "transform":
				options.Migrate = func(_ context.Context, path string) error {
					put(t, filepath.Join(path, "settings.json"), []byte("changed"))
					return sentinel
				}
			case "validate":
				options.Validate = func(context.Context, string) error { return sentinel }
			case "publish":
				options.beforePublish = func(string) error { return sentinel }
			case "cancel":
				options.Validate = func(context.Context, string) error { cancel(); return nil }
			case "source-changed":
				options.Validate = func(context.Context, string) error {
					put(t, filepath.Join(root, "subscriptions/new/metadata.json"), []byte(`{}`))
					return nil
				}
			}
			if _, err := Prepare(ctx, root, options); err == nil {
				t.Fatal("failure published a workspace")
			}
			target, _ := Directory(root, 1)
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("partial target exists", err)
			}
			after, _ := os.ReadFile(filepath.Join(root, "settings.json"))
			if !bytes.Equal(before, after) {
				t.Fatal("migration changed original")
			}
			stages, _ := filepath.Glob(filepath.Join(root, "config-workspaces", ".schema-1-stage-*"))
			if len(stages) != 0 {
				t.Fatal("failed transaction left its staging directory")
			}
			if _, err := Prepare(context.Background(), root, Options{Validate: valid}); err != nil {
				t.Fatal("retry after rollback failed", err)
			}
		})
	}
}

func TestInterruptedAndIncompleteWorkspacesAreNeverAdopted(t *testing.T) {
	root := fixture(t)
	put(t, filepath.Join(root, "config-workspaces/.schema-1-stage-crashed/settings.json"), []byte("partial"))
	workspace, err := Prepare(context.Background(), root, Options{Validate: valid})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(workspace, "settings.json"))
	if string(raw) == "partial" {
		t.Fatal("adopted interrupted stage")
	}
	if err := os.Remove(filepath.Join(workspace, "workspace.json")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(workspace, "settings.json"), []byte("do-not-overwrite"))
	if _, err := Prepare(context.Background(), root, Options{Validate: valid}); err == nil {
		t.Fatal("incomplete workspace was overwritten")
	}
	raw, _ = os.ReadFile(filepath.Join(workspace, "settings.json"))
	if string(raw) != "do-not-overwrite" {
		t.Fatal("damaged workspace overwritten")
	}
}

func TestFutureSchemaRequiresExplicitMigrationAndPublicationNeverOverwrites(t *testing.T) {
	root := fixture(t)
	if _, err := Prepare(context.Background(), root, Options{Version: 2, Validate: valid}); err == nil {
		t.Fatal("new schema silently copied legacy data without a migration")
	}
	target, _ := Directory(root, 1)
	_, err := Prepare(context.Background(), root, Options{Validate: valid, beforePublish: func(string) error {
		put(t, filepath.Join(target, "settings.json"), []byte("another-writer"))
		return nil
	}})
	if err == nil {
		t.Fatal("publication overwrote another workspace")
	}
	raw, _ := os.ReadFile(filepath.Join(target, "settings.json"))
	if string(raw) != "another-writer" {
		t.Fatal("publication removed another writer's data")
	}
}

func TestMigrationRejectsLinkedSources(t *testing.T) {
	root := fixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "local-scripts")); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if _, err := Prepare(context.Background(), root, Options{Validate: valid}); err == nil {
		t.Fatal("migration followed linked data")
	}
}
