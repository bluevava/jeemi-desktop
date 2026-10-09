package subscription

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type refreshFetchFunc func(context.Context, string) (FetchedContent, error)

func (f refreshFetchFunc) Fetch(ctx context.Context, address string) (FetchedContent, error) {
	return f(ctx, address)
}

func TestRefreshFetchRetriesOnceAndPreservesFailedRevision(t *testing.T) {
	for _, scenario := range []string{"primary success", "direct success", "primary timeout", "same content", "unavailable", "both fail", "invalid direct content", "cancel primary", "cancel direct", "cancel before"} {
		t.Run(scenario, func(t *testing.T) {
			store, setNow := newTestStore(t)
			oldSource := "mode: rule\n"
			created, err := store.Create(createInput{
				Name: "Saved name", SourceKind: SourceURL, ImportMethod: ImportURL,
				SourceURL: "https://example.invalid/sub?token=fixture",
				Format:    FormatYAML, Contents: []byte(oldSource),
				RemoteProfile: RemoteProfile{HasTraffic: true, DownloadBytes: 10, TotalBytes: 100},
			})
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Get(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			setNow(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			primaryCalls, factoryCalls, directCalls := 0, 0, 0
			fresh := FetchedContent{Contents: []byte("mode: global\n"), Format: FormatYAML, SuggestedName: "Do not rename",
				RemoteProfile: RemoteProfile{HasTraffic: true, DownloadBytes: 50, TotalBytes: 100}}
			if scenario == "same content" {
				fresh.Contents = []byte(oldSource)
			}
			manager, err := NewManager(ManagerOptions{
				Store: store,
				Fetcher: refreshFetchFunc(func(ctx context.Context, address string) (FetchedContent, error) {
					primaryCalls++
					if address != before.SourceURL {
						t.Error("primary request changed the saved URL")
					}
					switch scenario {
					case "primary success":
						return fresh, nil
					case "primary timeout":
						return FetchedContent{}, context.DeadlineExceeded
					case "cancel primary":
						cancel()
					}
					return FetchedContent{}, errors.New("primary failed")
				}),
				RefreshFallback: func() Fetcher {
					factoryCalls++
					if primaryCalls != 1 {
						t.Error("alternate route was resolved before primary failure")
					}
					if scenario == "unavailable" {
						return nil
					}
					return refreshFetchFunc(func(ctx context.Context, address string) (FetchedContent, error) {
						directCalls++
						if address != before.SourceURL {
							t.Error("retry changed the saved URL")
						}
						switch scenario {
						case "both fail":
							return FetchedContent{}, errors.New("direct failed")
						case "invalid direct content":
							return FetchedContent{Contents: []byte("not a configuration"), Format: FormatYAML}, nil
						case "cancel direct":
							cancel()
						}
						return fresh, nil
					})
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "cancel before" {
				cancel()
			}
			_, err = manager.Refresh(ctx, created.ID)
			succeeded := scenario == "primary success" || scenario == "direct success" || scenario == "primary timeout" || scenario == "same content"
			if (err == nil) != succeeded {
				t.Fatalf("unexpected refresh outcome: %v", err)
			}
			wantPrimary, wantFactory, wantDirect := 1, 1, 1
			switch scenario {
			case "cancel before":
				wantPrimary, wantFactory, wantDirect = 0, 0, 0
			case "primary success", "cancel primary":
				wantFactory, wantDirect = 0, 0
			case "unavailable":
				wantDirect = 0
			}
			if primaryCalls != wantPrimary || factoryCalls != wantFactory || directCalls != wantDirect {
				t.Fatalf("unexpected attempts: %d / %d / %d", primaryCalls, factoryCalls, directCalls)
			}
			after, err := store.Get(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !succeeded {
				if !reflect.DeepEqual(before, after) {
					t.Fatal("failed refresh changed saved metadata or revisions")
				}
				return
			}
			wantRevisions := before.RevisionCount + 1
			if scenario == "same content" {
				wantRevisions = before.RevisionCount
			}
			text, err := store.Text(created.ID)
			if err != nil || text.Contents != string(fresh.Contents) || after.RevisionCount != wantRevisions || after.LastFetchedAt == before.LastFetchedAt || after.RemoteProfile.DownloadBytes != 50 {
				t.Fatal("successful refresh did not commit the response and metadata")
			}
			if after.Name != before.Name || after.SourceURL != before.SourceURL {
				t.Fatal("refresh overwrote saved name or URL")
			}
		})
	}
}

func TestRefreshFallbackDoesNotApplyToImportsOrURLEdits(t *testing.T) {
	store, _ := newTestStore(t)
	fetcher := &queuedFetcher{results: []FetchedContent{{Contents: []byte("mode: rule\n"), Format: FormatYAML}}}
	manager, err := NewManager(ManagerOptions{Store: store, Fetcher: fetcher, RefreshFallback: func() Fetcher {
		t.Error("refresh retry was used for another operation")
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	state, err := manager.ImportURL(context.Background(), ImportURLInput{SourceURL: "https://example.invalid/first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ImportURL(context.Background(), ImportURLInput{SourceURL: "https://example.invalid/second"}); err == nil {
		t.Fatal("failed import succeeded")
	}
	if _, err := manager.Update(context.Background(), UpdateInput{ID: state.Subscriptions[0].ID, Name: "Edited", SourceURL: "https://example.invalid/third"}); err == nil {
		t.Fatal("failed URL edit succeeded")
	}
}
