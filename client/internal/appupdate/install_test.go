package appupdate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func replacementFixture(t *testing.T) (*replacement, string) {
	t.Helper()
	target, _ := targetFor("windows", "amd64")
	jobRoot := t.TempDir()
	packageFixture(t, filepath.Join(jobRoot, "extracted"), target, "1.2.3", []byte("new gui"))
	root := t.TempDir()
	executable := filepath.Join(root, target.executable)
	for _, name := range target.installItems() {
		if err := os.WriteFile(filepath.Join(root, name), []byte("old "+name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(root, "keep.txt"), []byte("user file"), 0600)
	digest, _, _ := hashFile(executable)
	job := installJob{ID: strings.Repeat("a", 32), Executable: executable, CurrentSHA256: digest, Version: "v1.2.3"}
	r, err := prepareReplacement(context.Background(), jobRoot, job, target)
	if err != nil {
		t.Fatal(err)
	}
	return r, root
}

func TestReplacementPairsFilesAndPreservesOtherContents(t *testing.T) {
	r, root := replacementFixture(t)
	if data, _ := os.ReadFile(filepath.Join(root, "Jeemi.exe")); string(data) != "old Jeemi.exe" {
		t.Fatal("prepare changed installed app")
	}
	if err := completeUpdate(r, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"Jeemi.exe": "new gui", "jeemi-authorizer.exe": "new helper", "keep.txt": "user file"} {
		data, _ := os.ReadFile(filepath.Join(root, name))
		if string(data) != want {
			t.Fatalf("%s: %q", name, data)
		}
	}
	r.cleanup()
	if _, err := os.Stat(r.stage); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestReplacementFailureAndRestartFailureRestoreEntirePair(t *testing.T) {
	for _, failRestart := range []bool{false, true} {
		r, root := replacementFixture(t)
		want := ErrRestart
		if !failRestart {
			want = ErrReplace
			r.rename = func(from, to string) error {
				if from == filepath.Join(r.stage, "new", "jeemi-authorizer.exe") {
					return errors.New("file occupied")
				}
				return os.Rename(from, to)
			}
		}
		err := completeUpdate(r, func() error { return ErrRestart })
		if err != want {
			t.Fatalf("%v want %v", err, want)
		}
		for _, name := range r.items {
			data, _ := os.ReadFile(filepath.Join(root, name))
			if string(data) != "old "+name {
				t.Fatalf("%s not restored: %q", name, data)
			}
		}
	}
}

func TestUnstoppedChildRetainsBackups(t *testing.T) {
	r, _ := replacementFixture(t)
	if err := completeUpdate(r, func() error { return ErrRollback }); err != ErrRollback {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.stage, "old", "Jeemi.exe")); err != nil {
		t.Fatal("backup lost", err)
	}
}

func TestWorkerRequiresCommitAndParentExit(t *testing.T) {
	for _, value := range []string{"", "commit", "cancel\n", "commit\nextra"} {
		if committedExit(strings.NewReader(value)) {
			t.Fatal(value)
		}
	}
	reader, writer := io.Pipe()
	done := make(chan bool, 1)
	go func() { done <- committedExit(reader) }()
	_, _ = io.WriteString(writer, "commit\n")
	select {
	case <-done:
		t.Fatal("replacement before parent exit")
	case <-time.After(20 * time.Millisecond):
	}
	_ = writer.Close()
	select {
	case accepted := <-done:
		if !accepted {
			t.Fatal("commit rejected")
		}
	case <-time.After(time.Second):
		t.Fatal("exit not observed")
	}
}

func TestManagerRejectsUnconfirmedOrExpiredUpdateWithoutIO(t *testing.T) {
	m := NewManager(t.TempDir(), "1.2.2")
	if err := m.Prepare(context.Background(), "v1.2.3"); err != ErrExpired {
		t.Fatal(err)
	}
	m.checked = Result{LatestVersion: "v1.2.3", UpdateAvailable: true}
	m.checkedAt = time.Now().Add(-time.Hour)
	if err := m.Prepare(context.Background(), "v1.2.3"); err != ErrExpired {
		t.Fatal(err)
	}
	m.checkedAt = time.Now()
	if err := m.Prepare(context.Background(), "v1.2.3"); err != ErrAssetUnavailable {
		t.Fatal(err)
	}
	if m.State().Phase != "failed" || m.State().Error != ErrAssetUnavailable.Error() {
		t.Fatal("error incorrectly marked cancelled", m.State())
	}
}

func TestRestartReceiptReleasesBusyState(t *testing.T) {
	m := NewManager(t.TempDir(), "1.2.3")
	if err := os.MkdirAll(m.root, 0700); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("b", 32)
	m.state = State{Phase: "restarting", JobID: id}
	receipt := State{Phase: "succeeded", Version: "v1.2.3", JobID: id}
	writeReceipt(m.root, receipt)
	if state := m.State(); state != receipt || busy(m.state.Phase) {
		t.Fatal(state)
	}
	data, _ := json.Marshal(receipt)
	if strings.Contains(string(data), "executable") {
		t.Fatal("exposed path")
	}
	m.DismissResult(id)
	if m.State().Phase != "idle" {
		t.Fatal("receipt repeated", m.State())
	}
	if NewManager(filepath.Dir(filepath.Dir(m.root)), "1.2.3").State().Phase != "idle" {
		t.Fatal("receipt repeated after reopen")
	}
}
