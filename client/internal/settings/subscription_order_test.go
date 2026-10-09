package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSubscriptionOrderPersistsWithoutOverwritingOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"22222222222222222222222222222222", "11111111111111111111111111111111"}
	if _, err := store.SetSelectedSubscription(ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSubscriptionOrder(ids); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSubscriptionPreferences(SelectorDensityLarge, 4, ConnectionResetOff); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	reopened, _ := NewStore(path)
	document, err := reopened.Load()
	if err != nil || !reflect.DeepEqual(document.Subscriptions.Order, ids) || document.Subscriptions.SelectedID != ids[1] || document.Subscriptions.SelectorDensity != SelectorDensityLarge {
		t.Fatalf("order/other settings were not preserved: %+v, %v", document.Subscriptions, err)
	}
	for _, invalid := range [][]string{{ids[0], ids[0]}, {"../outside"}} {
		if err := store.SetSubscriptionOrder(invalid); err == nil {
			t.Fatal("accepted invalid order")
		}
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("reads or rejected reorder rewrote settings")
	}
}

func TestSubscriptionOrderRejectsCorruptStoredOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	contents := `{"subscriptions":{"order":["invalid"]}}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(path)
	if _, err := store.Load(); err == nil {
		t.Fatal("accepted corrupt subscription order")
	}
	after, _ := os.ReadFile(path)
	if string(after) != contents {
		t.Fatal("corrupt settings were rewritten")
	}
}
