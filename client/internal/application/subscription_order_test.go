package application

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/subscription"
)

func TestSubscriptionOrderSurvivesReopenAddDeleteAndKeepsSelection(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	importFile := func(name string) subscription.State {
		t.Helper()
		path := filepath.Join(t.TempDir(), name+".yaml")
		if err := os.WriteFile(path, []byte("proxies: []\nrules: [MATCH,DIRECT]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		state, err := s.ImportSubscriptionFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	first := importFile("first").Subscriptions[0].ID
	second := importFile("second").Subscriptions[1].ID
	third := importFile("third").Subscriptions[2].ID
	want := []string{third, first, second}
	configuration := s.RuntimeStatus().Configuration
	if err := s.ReorderSubscriptions(want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.RuntimeStatus().Configuration, configuration) {
		t.Fatal("reordering reconciled runtime configuration")
	}
	assertOrder := func(state subscription.State, ids []string) {
		t.Helper()
		var got []string
		for _, item := range state.Subscriptions {
			got = append(got, item.ID)
		}
		if !reflect.DeepEqual(got, ids) || state.SelectedSubscriptionID != first {
			t.Fatalf("order/selection = %v/%s, want %v/%s", got, state.SelectedSubscriptionID, ids, first)
		}
	}
	reopened, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := reopened.SubscriptionState()
	if err != nil {
		t.Fatal(err)
	}
	assertOrder(state, want)
	for _, invalid := range [][]string{{third, first}, {third, first, first}, {third, first, "44444444444444444444444444444444"}} {
		if err := s.ReorderSubscriptions(invalid); err == nil {
			t.Fatal("accepted incomplete/duplicate/stale order")
		}
	}
	state = importFile("new")
	want = append(want, state.Subscriptions[3].ID)
	assertOrder(state, want)
	state, err = s.DeleteSubscription(second)
	if err != nil {
		t.Fatal(err)
	}
	assertOrder(state, []string{third, first, want[3]})
}
