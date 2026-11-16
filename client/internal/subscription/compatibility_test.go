package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"jeemi/internal/configfile"
)

func TestFutureSubscriptionFieldsSurviveEditsAndRefresh(t *testing.T) {
	root, store, detail, path := recoverySubscription(t, SourceURL)
	original := changeMetadata(t, path, func(m map[string]json.RawMessage) {
		m["futurePreference"] = json.RawMessage(`{"enabled":true,"large":9007199254740993}`)
		var revisions []map[string]json.RawMessage
		json.Unmarshal(m["revisions"], &revisions)
		revisions[0]["futureRevision"] = json.RawMessage(`"keep"`)
		m["revisions"], _ = json.Marshal(revisions)
	})
	if err := CheckStartupConfiguration(root); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("compatible read wrote metadata")
	}
	input := UpdateInput{ID: detail.ID, Name: "changed", SourceURL: detail.SourceURL, AutoUpdateIntervalMinutes: 180}
	for i := 0; i < 3; i++ {
		if _, err := store.UpdateMetadata(input); err != nil {
			t.Fatal(err)
		}
		if _, err := store.UpdateFromURL(input, FetchedContent{Format: FormatYAML, Contents: []byte("proxies: []\n# refreshed\n")}); err != nil {
			t.Fatal(err)
		}
		store, _ = NewStore(StoreOptions{DataDirectory: root})
		raw, _ := os.ReadFile(path)
		if !bytes.Contains(raw, []byte("9007199254740993")) || !bytes.Contains(raw, []byte("futureRevision")) {
			t.Fatal("refresh dropped newer metadata")
		}
		if _, err := store.Text(detail.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFutureSubscriptionFormatCannotBeResetOrRepaired(t *testing.T) {
	for _, field := range []string{"manifestVersion", "requiredFeatures"} {
		t.Run(field, func(t *testing.T) {
			root, _, _, path := recoverySubscription(t, SourceURL)
			before := changeMetadata(t, path, func(m map[string]json.RawMessage) {
				if field == "manifestVersion" {
					m[field] = json.RawMessage(`99`)
				} else {
					m[field] = json.RawMessage(`["future-routing"]`)
				}
			})
			err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(*configfile.Failure) (bool, error) {
				t.Fatal("unsupported metadata offered for repair or reset")
				return true, nil
			})
			if !configfile.IsUnsupported(err) {
				t.Fatal("future format not classified", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("future metadata changed")
			}
		})
	}
}

func TestConfirmedRepairRetainsUnknownOptionalFields(t *testing.T) {
	root, _, _, path := recoverySubscription(t, SourceURL)
	changeMetadata(t, path, func(m map[string]json.RawMessage) {
		m["autoUpdateIntervalMinutes"] = json.RawMessage(`"broken"`)
		m["futurePreference"] = json.RawMessage(`{"keep":true}`)
	})
	if err := configfile.Recover(context.Background(), root, func() error { return CheckStartupConfiguration(root) }, func(f *configfile.Failure) (bool, error) {
		if !f.CanRepair() {
			t.Fatal("expected repair")
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !bytes.Contains(raw, []byte("futurePreference")) {
		t.Fatal("repair removed unknown fields")
	}
}
