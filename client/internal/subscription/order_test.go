package subscription

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestStoreListsByCreationTimeAfterRefreshAndEdit(t *testing.T) {
	store, setNow := newTestStore(t)
	nextID := 3
	store.newID = func() (string, error) {
		id := fmt.Sprintf("%032x", nextID)
		nextID--
		return id, nil
	}
	base := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	var ids []string
	for index, name := range []string{"Zulu", "Beta", "Alpha"} {
		setNow(base.Add(time.Duration(index) * 100 * time.Millisecond))
		item, err := store.Create(createInput{Name: name, SourceKind: SourceURL, ImportMethod: ImportURL,
			SourceURL: "https://example.invalid/sub", Format: FormatYAML, Contents: []byte("mode: rule\n")})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}
	setNow(base.Add(time.Hour))
	if _, err := store.UpdateFromURL(UpdateInput{ID: ids[0], Name: "Renamed", SourceURL: "https://example.invalid/sub"},
		FetchedContent{Format: FormatYAML, Contents: []byte("mode: global\n")}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLocalConfig(ids[1], fixedSubscriptionID); err != nil {
		t.Fatal(err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, item := range state.Subscriptions {
		got = append(got, item.ID)
	}
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("refresh/edit changed creation order: %v, want %v", got, ids)
	}
}

func TestApplyOrderAppendsNewAndIgnoresDeletedIDs(t *testing.T) {
	items := []Summary{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "new"}}
	ordered := ApplyOrder(items, []string{"c", "deleted", "a", "b"})
	var got []string
	for _, item := range ordered {
		got = append(got, item.ID)
	}
	if !reflect.DeepEqual(got, []string{"c", "a", "b", "new"}) || items[0].ID != "a" {
		t.Fatalf("saved order did not preserve additions/source list: %v", got)
	}
}
