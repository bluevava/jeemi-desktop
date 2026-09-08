//go:build linux

package coreauth

import (
	"bytes"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

type fakeRevoke struct {
	value    []byte
	removed  int
	retained bool
}

func (f *fakeRevoke) capabilities(int) ([]byte, error) {
	if f.value == nil {
		return nil, unix.ENODATA
	}
	return f.value, nil
}
func (f *fakeRevoke) removeCapabilities(int) error {
	f.removed++
	if !f.retained {
		f.value = nil
	}
	return nil
}
func TestRevokeOnlyManagedCapabilitiesAndReadBack(t *testing.T) {
	target := fixture(t)
	for _, tc := range []struct {
		name     string
		value    []byte
		retained bool
		wantErr  bool
		writes   int
	}{
		{"absent", nil, false, false, 0},
		{"managed", networkCapabilities(), false, false, 1},
		{"foreign", []byte{1, 2, 3}, false, true, 0},
		{"failed readback", networkCapabilities(), true, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := &fakeRevoke{value: tc.value, retained: tc.retained}
			err := revoke(target, uint32(os.Getuid()), ops)
			if (err != nil) != tc.wantErr || ops.removed != tc.writes {
				t.Fatalf("%v writes=%d", err, ops.removed)
			}
		})
	}
}
func TestRemovePolicyIsIdempotentAndPreservesAdministratorEdits(t *testing.T) {
	dir := t.TempDir()
	uid := uint32(os.Getuid())
	p := makeResolverPolicy(uid, "test-user")
	for _, entry := range p.entries() {
		if err := os.WriteFile(filepath.Join(dir, entry.name), entry.data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.remove(dir, uid); err != nil {
		t.Fatal(err)
	}
	if err := p.remove(dir, uid); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("administrator edit")
	file := filepath.Join(dir, p.entries()[0].name)
	if err := os.WriteFile(file, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.remove(dir, uid); err == nil {
		t.Fatal("foreign policy removed")
	}
	actual, _ := os.ReadFile(file)
	if !bytes.Equal(actual, foreign) {
		t.Fatal("foreign policy changed")
	}
}
func TestCleanupProtocolRejectsCommandsAndAmbiguousRequests(t *testing.T) {
	for _, input := range []string{`{"operation":"exec","command":"id"}`, `{"operation":"remove"}`, `{"operation":"remove","targets":[],"executablePath":"/bin/sh"}`, `{"operation":"install","targets":[]}`} {
		if _, err := decodeHelperRequest(bytes.NewBufferString(input)); err == nil {
			t.Fatal(input)
		}
	}
	if _, err := decodeHelperRequest(bytes.NewBufferString(`{"operation":"remove","targets":[]}`)); err != nil {
		t.Fatal(err)
	}
}
