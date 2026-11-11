package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/configfile"
)

func recoverySubscription(t *testing.T, kind SourceKind) (string, *Store, Detail, string) {
	t.Helper()
	root := t.TempDir()
	store, err := NewStore(StoreOptions{DataDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	input := createInput{Name: "Recovery fixture", SourceKind: kind, Format: FormatYAML, Contents: []byte("proxies: []\n")}
	if kind == SourceURL {
		input.ImportMethod, input.SourceURL, input.AutoUpdateIntervalMinutes = ImportURL, "https://example.test/sub?token=fixture", 180
	} else {
		input.ImportMethod, input.OriginalFileName = ImportFile, "fixture.yaml"
	}
	detail, err := store.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	return root, store, detail, filepath.Join(store.Directory(), detail.ID, "metadata.json")
}

func changeMetadata(t *testing.T, path string, edit func(map[string]json.RawMessage)) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(contents, &fields); err != nil {
		t.Fatal(err)
	}
	edit(fields)
	contents, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestStartupRepairPreservesSubscriptionSourceAndValidPreferences(t *testing.T) {
	for _, test := range []struct {
		name     string
		kind     SourceKind
		interval int
		edit     func(map[string]json.RawMessage)
	}{
		{"unknown fields", SourceURL, 180, func(m map[string]json.RawMessage) { m["futurePreference"] = json.RawMessage(`{"enabled":true}`) }},
		{"wrong interval type", SourceURL, 60, func(m map[string]json.RawMessage) { m["autoUpdateIntervalMinutes"] = json.RawMessage(`"incompatible"`) }},
		{"invalid interval", SourceURL, 60, func(m map[string]json.RawMessage) { m["autoUpdateIntervalMinutes"] = json.RawMessage(`5`) }},
		{"missing fallback", SourceURL, 180, func(m map[string]json.RawMessage) { delete(m, "fallback") }},
		{"file interval", SourceFile, 0, func(m map[string]json.RawMessage) { m["autoUpdateIntervalMinutes"] = json.RawMessage(`60`) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, store, before, path := recoverySubscription(t, test.kind)
			original := changeMetadata(t, path, test.edit)
			cached, err := store.Text(before.ID)
			if err == nil {
				t.Fatal("ordinary reads must not silently accept incompatible metadata", cached)
			}
			err = CheckStartupConfiguration(root)
			var failure *configfile.Failure
			if !errors.As(err, &failure) || !failure.CanRepair() || failure.Path != path {
				t.Fatal("repair was not offered", err)
			}
			unchanged, _ := os.ReadFile(path)
			if !bytes.Equal(unchanged, original) {
				t.Fatal("startup check wrote metadata")
			}
			confirmations := 0
			err = configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
				confirmations++
				if !f.CanRepair() {
					t.Fatal("repair changed into deletion")
				}
				return true, nil
			})
			if err != nil || confirmations != 1 {
				t.Fatal(err, confirmations)
			}
			after, err := store.Get(before.ID)
			before.AutoUpdateIntervalMinutes = test.interval
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatal("repair changed source identity, revisions or valid preferences", err)
			}
			text, err := store.Text(before.ID)
			if err != nil || text.Contents != "proxies: []\n" {
				t.Fatal("cached source changed", err)
			}
			backups, _ := filepath.Glob(path + ".before-repair-*")
			if len(backups) != 1 {
				t.Fatal("missing original backup")
			}
			backup, _ := os.ReadFile(backups[0])
			if !bytes.Equal(backup, original) {
				t.Fatal("original backup changed")
			}
		})
	}
}

func TestStartupResetRemovesOnlyUnrepairableMetadata(t *testing.T) {
	for _, corruption := range []string{"syntax", "version", "identity", "revision", "source"} {
		t.Run(corruption, func(t *testing.T) {
			root, store, before, path := recoverySubscription(t, SourceURL)
			changeMetadata(t, path, func(m map[string]json.RawMessage) {
				switch corruption {
				case "version":
					m["manifestVersion"] = json.RawMessage(`99`)
				case "identity":
					m["id"] = json.RawMessage(`"not-the-subscription"`)
				case "revision":
					m["revisions"] = json.RawMessage(`[]`)
				case "source":
					m["sourceUrl"] = json.RawMessage(`"file:///untrusted"`)
				}
			})
			if corruption == "syntax" {
				if err := os.WriteFile(path, []byte(`{"name":`), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
				if f.CanRepair() || f.Path != path {
					t.Fatal("unsafe repair offered")
				}
				return true, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.State()
			if err != nil || len(state.Subscriptions) != 0 {
				t.Fatal("reset subscription still blocks the list", err)
			}
			cached := filepath.Join(store.Directory(), before.ID, "revisions", before.CurrentRevisionID, "source.yaml")
			contents, err := os.ReadFile(cached)
			if err != nil || string(contents) != "proxies: []\n" {
				t.Fatal("reset removed cached source", err)
			}
			if _, err := store.Create(createInput{Name: "Reimported", SourceKind: SourceURL, ImportMethod: ImportURL, SourceURL: before.SourceURL, Format: FormatYAML, Contents: contents}); err != nil {
				t.Fatal("could not reimport after reset", err)
			}
		})
	}
}

func TestStartupSubscriptionCheckKeepsSupportedOptionalFieldsWithoutWriting(t *testing.T) {
	root, store, detail, path := recoverySubscription(t, SourceURL)
	original := changeMetadata(t, path, func(m map[string]json.RawMessage) { delete(m, "autoUpdateIntervalMinutes") })
	if err := CheckStartupConfiguration(root); err != nil {
		t.Fatal(err)
	}
	after, err := store.Get(detail.ID)
	if err != nil || after.AutoUpdateIntervalMinutes != 60 {
		t.Fatal("omitted interval did not use the current default", err)
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, original) {
		t.Fatal("valid optional fields were written back")
	}
}

func TestStartupSubscriptionReadFailureNeverOffersReset(t *testing.T) {
	root, _, _, path := recoverySubscription(t, SourceURL)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(*configfile.Failure) (bool, error) {
		t.Fatal("unreadable metadata offered for deletion")
		return true, nil
	})
	var failure *configfile.Failure
	if !errors.As(err, &failure) || failure.Reason != "read" {
		t.Fatal(err)
	}
}
