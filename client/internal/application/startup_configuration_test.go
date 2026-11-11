package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"jeemi/internal/configfile"
	"jeemi/internal/subscription"
)

func TestStartupCheckDoesNotCreateData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-data")
	if err := CheckStartupConfiguration(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("check created data", err)
	}
}

func TestStartupRecoveryUsesActualReadersAndKeepsUnrelatedData(t *testing.T) {
	root := t.TempDir()
	paths := []string{"settings.json", "local-configs/resources/library.json", "chain-proxies/library.json", "geodata/manifest.json"}
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"invalid":`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(root, "notes.json")
	if err := os.WriteFile(keep, []byte("keep this file"), 0o600); err != nil {
		t.Fatal(err)
	}
	count := 0
	err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
		if count >= len(paths) || f.Path != filepath.Join(root, filepath.FromSlash(paths[count])) || f.Reason != "invalid" {
			t.Fatalf("unexpected failure: %+v", f)
		}
		count++
		return true, nil
	})
	if err != nil || count != len(paths) {
		t.Fatal(err, count)
	}
	if _, err := NewService(root); err != nil {
		t.Fatal("startup still failed after recovery", err)
	}
	contents, err := os.ReadFile(keep)
	if err != nil || string(contents) != "keep this file" {
		t.Fatal("unrelated data changed", err)
	}
}

func TestStartupCheckIdentifiesSemanticErrorsWithoutWriting(t *testing.T) {
	for _, test := range []struct{ path, contents string }{
		{"local-configs/resources/library.json", `{"version":99,"revision":1}`},
		{"local-configs/resources/library.json", `{"version":1,"revision":1,"updatedAt":"invalid"}`},
		{"geodata/manifest.json", `{"version":99}`},
		{"chain-proxies/library.json", `{"version":99,"revision":1,"groups":[]}`},
		{"geodata/manifest.json", `{"version":1,"desired":{"geoip":"invalid"}}`},
		{"settings.json", `{"runtime":{"listenPort":"invalid"}}`},
	} {
		t.Run(test.path+test.contents, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			err := CheckStartupConfiguration(root)
			var failure *configfile.Failure
			if !errors.As(err, &failure) || failure.Path != path || failure.Reason != "invalid" {
				t.Fatalf("missing file diagnostic: %v", err)
			}
			contents, _ := os.ReadFile(path)
			if string(contents) != test.contents {
				t.Fatal("read modified the file")
			}
		})
	}
}

func TestStartupSubscriptionRecoveryResumesOfflineComposition(t *testing.T) {
	for _, test := range []struct {
		name            string
		repair, healthy bool
	}{{"repair", true, true}, {"reset", false, true}, {"reset last subscription", false, false}} {
		t.Run(test.name, func(t *testing.T) {
			service, fetcher, _, selectedID := scheduledService(t)
			var state subscription.State
			if test.healthy {
				var err error
				state, err = service.ImportSubscriptionURL(subscription.ImportURLInput{Name: "Healthy subscription", SourceURL: "https://example.test/healthy"})
				if err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Dir(service.subscriptions.Directory())
			path := filepath.Join(service.subscriptions.Directory(), selectedID, "metadata.json")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var metadata map[string]json.RawMessage
			if err := json.Unmarshal(contents, &metadata); err != nil {
				t.Fatal(err)
			}
			metadata["futurePreference"] = json.RawMessage(`true`)
			if !test.repair {
				metadata["manifestVersion"] = json.RawMessage(`99`)
			}
			contents, err = json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			requests := fetcher.calls.Load()
			err = configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
				if f.Path != path || f.CanRepair() != test.repair {
					t.Fatal("wrong subscription recovery action", f.Path)
				}
				return true, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if fetcher.calls.Load() != requests {
				t.Fatal("startup repair fetched a subscription")
			}
			recovered, err := NewService(root)
			if err != nil {
				t.Fatal("service construction failed after recovery", err)
			}
			after, err := recovered.SubscriptionState()
			if err != nil {
				t.Fatal("subscription list failed after recovery", err)
			}
			wantCount, wantID := 1, selectedID
			if test.healthy {
				wantCount++
			}
			if !test.repair {
				wantCount--
				wantID = ""
				for _, item := range state.Subscriptions {
					if item.ID != selectedID {
						wantID = item.ID
					}
				}
			}
			if len(after.Subscriptions) != wantCount || after.SelectedSubscriptionID != wantID {
				t.Fatal("recovery lost the valid subscription or kept an invalid selection")
			}
			_, err = recovered.buildRuntimeCandidate()
			if wantCount == 0 && !errors.Is(err, errNoSelectedSubscription) {
				t.Fatal("empty subscription list did not recover to an empty selection", err)
			} else if wantCount > 0 && err != nil {
				t.Fatal("cached subscription cannot compose after recovery", err)
			}
		})
	}
}
