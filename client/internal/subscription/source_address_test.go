package subscription

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceAddressUsesURLForURLAndQRImports(t *testing.T) {
	for _, method := range []ImportMethod{ImportURL, ImportQRImage, ImportQRScreen} {
		t.Run(string(method), func(t *testing.T) {
			store, _ := newTestStore(t)
			address := "https://account:fixture@example.invalid/sub?token=fixture-token"
			detail, err := store.Create(createInput{Name: "Remote", SourceKind: SourceURL, ImportMethod: method,
				SourceURL: address, Format: FormatYAML, Contents: []byte("mode: rule\n")})
			if err != nil {
				t.Fatal(err)
			}
			copied, err := store.SourceAddress(detail.ID)
			if err != nil || copied != address {
				t.Fatal("explicit copy did not preserve the complete saved URL")
			}
			if detail.SourceLabel != "https://example.invalid" {
				t.Fatal("list source exposed credentials")
			}
		})
	}
}

func TestFileSourceAddressUsesVerifiedManagedRevision(t *testing.T) {
	store, _ := newTestStore(t)
	contents := []byte("mode: rule\n")
	detail, err := store.Create(createInput{Name: "Local", SourceKind: SourceFile, ImportMethod: ImportFile,
		OriginalFileName: "original.yaml", Format: FormatYAML, Contents: contents})
	if err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(store.Directory(), detail.ID, "metadata.json")
	before, _ := os.ReadFile(metadataPath)
	address, err := store.SourceAddress(detail.ID)
	want := filepath.Join(store.Directory(), detail.ID, "revisions", detail.CurrentRevisionID, "source.yaml")
	if err != nil || address != want || !filepath.IsAbs(address) {
		t.Fatalf("copy did not resolve the managed current revision: %v", err)
	}
	stored, err := os.ReadFile(address)
	if err != nil || string(stored) != string(contents) {
		t.Fatal("copied address did not point to the saved source")
	}
	after, _ := os.ReadFile(metadataPath)
	if string(before) != string(after) {
		t.Fatal("copying a path rewrote metadata")
	}
	if err := os.WriteFile(address, []byte("mode: direct\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SourceAddress(detail.ID); err == nil {
		t.Fatal("accepted a damaged current revision")
	}
	if err := os.Remove(address); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SourceAddress(detail.ID); err == nil {
		t.Fatal("accepted a missing current revision")
	}
	if _, err := store.SourceAddress("../outside"); err == nil {
		t.Fatal("accepted an arbitrary path as a subscription ID")
	}
}
