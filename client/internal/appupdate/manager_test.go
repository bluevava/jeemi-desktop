package appupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcknowledgeRestartIdentifiesConfirmedUpdate(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self = canonicalTestPath(t, self)
	id := strings.Repeat("d", 32)
	for _, name := range []string{
		"confirmed", "ordinary startup", "invalid argument", "invalid ID",
		"extra argument", "missing job", "invalid job", "different job",
		"different executable", "acknowledgement write failure",
	} {
		t.Run(name, func(t *testing.T) {
			manager := NewManager(t.TempDir(), "1.2.3")
			jobRoot := filepath.Join(manager.root, id)
			if err := os.MkdirAll(jobRoot, 0700); err != nil {
				t.Fatal(err)
			}
			job := installJob{ID: id, Executable: self, Version: "v1.2.3"}
			previousArgs := os.Args
			t.Cleanup(func() { os.Args = previousArgs })
			os.Args = []string{self, "--jeemi-update-restarted", id}
			switch name {
			case "ordinary startup":
				os.Args = []string{self}
			case "invalid argument":
				os.Args[1] = "--unrelated"
			case "invalid ID":
				os.Args[2] = "../outside"
			case "extra argument":
				os.Args = append(os.Args, "extra")
			case "different job":
				job.ID = strings.Repeat("e", 32)
			case "different executable":
				job.Executable = filepath.Join(jobRoot, "other-client")
			case "acknowledgement write failure":
				if err := os.Mkdir(filepath.Join(jobRoot, "started"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if name != "missing job" {
				data, err := json.Marshal(job)
				if err != nil {
					t.Fatal(err)
				}
				if name == "invalid job" {
					data = []byte("{")
				}
				if err := os.WriteFile(filepath.Join(jobRoot, "job.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := name == "confirmed"
			if got := manager.AcknowledgeRestart(); got != want {
				t.Fatalf("update restart = %v, want %v", got, want)
			}
			data, err := os.ReadFile(filepath.Join(jobRoot, "started"))
			state := manager.State()
			if want {
				if err != nil || string(data) != "1.2.3" || state.JobID != id || state.Version != "v1.2.3" {
					t.Fatalf("confirmation = %q, %v; state = %+v", data, err, state)
				}
			} else if err == nil || state.JobID != "" {
				t.Fatalf("unconfirmed startup changed the update state: %q, %+v", data, state)
			}
		})
	}
}
