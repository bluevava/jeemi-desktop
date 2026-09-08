package uidiagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testFailure() Failure {
	return Failure{Kind: "render", Scope: "page", Page: "subscriptions", ErrorType: "TypeError", Code: "invalid_value", Frames: []Frame{{Asset: "index-Abc123_x.js", Line: 42, Column: 91}}}
}

func TestStoreBoundsHistoryAcrossRestarts(t *testing.T) {
	root := t.TempDir()
	s := New(root, "test-version")
	now := time.Now()
	s.now = func() time.Time { return now }
	if _, err := os.Stat(filepath.Join(root, "diagnostics")); !os.IsNotExist(err) {
		t.Fatal("creating the diagnostic store must not touch the filesystem")
	}
	for i := 0; i < 65; i++ {
		now = now.Add(time.Minute)
		input := testFailure()
		input.Frames[0].Line = i + 1
		if err := s.Report(input); err != nil {
			t.Fatal(err)
		}
	}
	s = New(root, "next-version")
	if err := s.Report(testFailure()); err != nil {
		t.Fatal(err)
	}
	records := readRecords(filepath.Join(root, "diagnostics", "ui-errors.json"))
	if len(records) != maxRecords || records[0].Failure.Frames[0].Line != 17 || records[49].AppVersion != "next-version" {
		t.Fatalf("unexpected retained diagnostic history: %#v", records)
	}
	entries, err := os.ReadDir(s.directory)
	if err != nil || len(entries) != 1 {
		t.Fatal("diagnostic writes left temporary files")
	}
}

func TestStoreRejectsUnstructuredOrSensitiveInput(t *testing.T) {
	s := New(t.TempDir(), "test")
	for _, change := range []func(*Failure){
		func(f *Failure) { f.Page = "config/private-subscription/edit?token=SECRET" },
		func(f *Failure) { f.ErrorType = "password=SECRET" },
		func(f *Failure) { f.Code = "https://example.invalid/?token=SECRET" },
		func(f *Failure) { f.Frames[0].Asset = "https://localhost/assets/index-Abc123_x.js?token=SECRET" },
		func(f *Failure) { f.Frames = make([]Frame, 6) },
	} {
		input := testFailure()
		change(&input)
		if err := s.Report(input); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatal("unsafe diagnostic accepted or echoed")
		}
	}
	if _, err := os.Stat(s.directory); !os.IsNotExist(err) {
		t.Fatal("invalid reports caused disk writes")
	}
}

func TestStoreRateLimitsAndRecoversFromCorruptFile(t *testing.T) {
	s := New(t.TempDir(), "test")
	now := time.Now()
	s.now = func() time.Time { return now }
	directory, err := s.Directory()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "ui-errors.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("X", maxFileBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxReportsPerMinute; i++ {
		if err := s.Report(testFailure()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Report(testFailure()); err == nil {
		t.Fatal("unbounded error storm writes")
	}
	if records := readRecords(path); len(records) != maxReportsPerMinute {
		t.Fatal("invalid history was not replaced with bounded records")
	}
	now = now.Add(time.Minute)
	if err := s.Report(testFailure()); err != nil {
		t.Fatal("rate limit did not reset")
	}
}

func TestStoreFailureDoesNotExposeLocalPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-data")
	if err := os.WriteFile(root, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	err := New(root, "test").Report(testFailure())
	if err == nil || strings.Contains(err.Error(), "private-data") {
		t.Fatalf("unexpected error: %v", err)
	}
}
