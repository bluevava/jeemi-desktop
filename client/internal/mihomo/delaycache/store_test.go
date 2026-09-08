package delaycache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testScope() Scope {
	return Scope{
		SubscriptionID:       strings.Repeat("a", 32),
		SubscriptionRevision: strings.Repeat("b", 64),
		LocalConfigID:        strings.Repeat("c", 32), LocalConfigRevision: 3,
	}
}

func TestStorePersistsAndMergesDelayResults(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "jeemi_data")
	store, err := NewStore(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time {
		return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	}
	scope := testScope()
	first, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A", Status: StatusSuccess, Delay: 82, Source: SourceManual},
		{Name: "Node B", Status: StatusError, Source: SourceManual},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Results) != 2 || first.Results[0].Name != "Node A" || first.Results[0].Delay != 82 {
		t.Fatalf("unexpected first snapshot: %+v", first)
	}
	store.now = func() time.Time {
		return time.Date(2026, 9, 2, 12, 1, 0, 0, time.UTC)
	}
	if _, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A", Status: StatusSuccess, Delay: 45, Source: SourceMihomo},
	}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Results) != 2 || loaded.Results[0].Delay != 45 || loaded.Results[0].Source != SourceMihomo {
		t.Fatalf("unexpected merged snapshot: %+v", loaded)
	}
	cachePath := filepath.Join(dataRoot, "cache", "mihomo", "proxy-delays", scope.SubscriptionID+".json")
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache permissions are too broad: %v", info.Mode().Perm())
	}
}

func TestStoreDoesNotReuseResultsAcrossProjectionRevisions(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "jeemi_data"))
	if err != nil {
		t.Fatal(err)
	}
	scope := testScope()
	if _, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A", Status: StatusSuccess, Delay: 82, Source: SourceManual},
	}}); err != nil {
		t.Fatal(err)
	}
	changed := scope
	changed.SubscriptionRevision = strings.Repeat("d", 64)
	loaded, err := store.Load(changed)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Results) != 0 || loaded.Scope != changed {
		t.Fatalf("stale cache escaped its projection scope: %+v", loaded)
	}
}

func TestStoreRejectsInvalidInputAndDeletesCache(t *testing.T) {
	dataRoot := filepath.Join(t.TempDir(), "jeemi_data")
	store, err := NewStore(dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	scope := testScope()
	if _, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A", Status: StatusSuccess, Delay: 0, Source: SourceManual},
	}}); err == nil {
		t.Fatal("Merge() accepted an invalid successful delay")
	}
	if _, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A\nsecret", Status: StatusError, Source: SourceManual},
	}}); err == nil {
		t.Fatal("Merge() accepted a control character in a node name")
	}
	if _, err := store.Merge(Update{Scope: scope, Results: []UpdateEntry{
		{Name: "Node A", Status: StatusSuccess, Delay: 82, Source: SourceManual},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(scope.SubscriptionID); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Results) != 0 {
		t.Fatalf("cache still exists after delete: %+v", loaded)
	}
}
