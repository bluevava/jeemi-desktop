package subscription

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticUpdateIntervalPersistsAndSurvivesRefresh(t *testing.T) {
	for _, minutes := range []int{0, 10, 30, 60, 180, 720} {
		store, _ := newTestStore(t)
		fetched := FetchedContent{Format: FormatYAML, Contents: []byte("mode: rule\n"), RemoteProfile: RemoteProfile{UpdateIntervalHours: 24}}
		fetcher := &queuedFetcher{results: []FetchedContent{fetched, fetched}}
		manager, _ := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
		state, err := manager.ImportURL(context.Background(), ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/sub", AutoUpdateIntervalMinutes: minutes})
		if err != nil {
			t.Fatal(err)
		}
		want := minutes
		if want == 0 {
			want = 60
		}
		id := state.Subscriptions[0].ID
		state, err = manager.Refresh(context.Background(), id)
		if err != nil || state.Subscriptions[0].AutoUpdateIntervalMinutes != want {
			t.Fatal("refresh or provider header overwrote local interval", err)
		}
		detail, _ := manager.Get(id)
		_, err = manager.Update(context.Background(), UpdateInput{ID: id, Name: detail.Name, SourceURL: detail.SourceURL, AutoUpdateIntervalMinutes: 180})
		if err != nil {
			t.Fatal(err)
		}
		reopened, _ := NewStore(StoreOptions{DataDirectory: filepath.Dir(store.Directory())})
		got, err := reopened.Get(id)
		if err != nil || got.AutoUpdateIntervalMinutes != 180 || len(fetcher.urls) != 2 {
			t.Fatal("metadata-only edit did not persist without fetching", err)
		}
	}
}

func TestAutomaticUpdateIntervalRejectsInvalidWithoutFetchingOrWriting(t *testing.T) {
	for _, minutes := range []int{-1, 1, 15, 59, 1440} {
		store, _ := newTestStore(t)
		fetcher := &queuedFetcher{}
		manager, _ := NewManager(ManagerOptions{Store: store, Fetcher: fetcher})
		_, err := manager.ImportURL(context.Background(), ImportURLInput{Name: "Fixture", SourceURL: "https://example.test/sub", AutoUpdateIntervalMinutes: minutes})
		if err == nil || len(fetcher.urls) != 0 {
			t.Fatal("invalid interval fetched or was accepted")
		}
	}
	store, _ := newTestStore(t)
	created, err := store.Create(createInput{Name: "File", SourceKind: SourceFile, ImportMethod: ImportFile, OriginalFileName: "fixture.yaml", Format: FormatYAML, Contents: []byte("mode: rule\n")})
	if err != nil || created.AutoUpdateIntervalMinutes != 0 {
		t.Fatal("file subscription scheduled", err)
	}
	path := filepath.Join(store.Directory(), created.ID, "metadata.json")
	before, _ := os.ReadFile(path)
	if _, err := store.UpdateMetadata(UpdateInput{ID: created.ID, Name: "Changed", AutoUpdateIntervalMinutes: 60}); err == nil {
		t.Fatal("file interval accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid interval changed metadata")
	}
}
