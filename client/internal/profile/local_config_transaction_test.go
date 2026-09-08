package profile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"jeemi/internal/config/resources"
)

func TestResourceCommitFailureRestoresExactConfigFiles(t *testing.T) {
	store := newTestLocalConfigStore(t)
	before, err := store.Save(SaveLocalConfigInput{Name: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, name := range []string{"manifest.json", "overlay.yaml", "merge-plan.json"} {
		data, err := os.ReadFile(filepath.Join(store.Directory(), before.ID, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	failure := errors.New("simulated library commit failure")
	_, err = store.SaveWithResourceCommit(SaveLocalConfigInput{ID: before.ID, Name: "Candidate"}, resources.State{}, func() error { return failure })
	if !errors.Is(err, failure) {
		t.Fatal("missing commit failure", err)
	}
	after, err := store.Get(before.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("previous revision not restored", err)
	}
	for name, expected := range files {
		data, err := os.ReadFile(filepath.Join(store.Directory(), before.ID, name))
		if err != nil || string(data) != expected {
			t.Fatal("rollback changed file", name, err)
		}
	}
	entries, _ := os.ReadDir(store.Directory())
	if len(entries) != 1 {
		t.Fatal("candidate or backup directory leaked")
	}
}
