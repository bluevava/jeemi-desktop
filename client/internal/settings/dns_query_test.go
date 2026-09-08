package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"jeemi/internal/dnsquery"
)

func TestDNSQueryPreferencesLoadWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := store.Load()
	if err != nil || document.DNSQuery == nil || *document.DNSQuery != dnsquery.DefaultPreferences() {
		t.Fatalf("defaults = %+v, %v", document.DNSQuery, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reading defaults wrote a settings file")
	}
	contents := []byte(`{"mihomo":{"selectedVersion":"v1.19.30"}}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(contents, after) {
		t.Fatal("reading a missing DNS preference rewrote settings")
	}
}

func TestDNSQueryPreferencesPreserveOtherSettingsAndExplicitEmptyAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetMihomoVersion("v1.19.30"); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"https://resolver.example/private?key=test", "2606:4700:4700::1111", "", "unfinished-address"} {
		if err := store.SetDNSQueryPreferences(dnsquery.Preferences{CustomDNS: "  " + address + "  "}); err != nil {
			t.Fatal(err)
		}
		reopened, err := NewStore(path)
		if err != nil {
			t.Fatal(err)
		}
		after, err := reopened.Load()
		if err != nil || after.DNSQuery == nil || after.DNSQuery.CustomDNS != address {
			t.Fatalf("address was not restored: %+v, %v", after.DNSQuery, err)
		}
		after.DNSQuery = before.DNSQuery
		if !reflect.DeepEqual(before, after) {
			t.Fatal("DNS preference changed other settings")
		}
	}
	if _, err := store.SetMihomoVersion("v1.19.31"); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil || after.DNSQuery.CustomDNS != "unfinished-address" {
		t.Fatal("another settings update lost the saved DNS address")
	}
}

func TestDNSQueryPreferencesRejectOversizedInputWithoutLosingSavedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetDNSQueryPreferences(dnsquery.Preferences{CustomDNS: "9.9.9.9"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{strings.Repeat("x", 2049), "dns\n.example", "dns\x00.example"} {
		if err := store.SetDNSQueryPreferences(dnsquery.Preferences{CustomDNS: input}); err == nil {
			t.Fatal("accepted an unbounded or control-character value")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("invalid preference replaced the saved address")
		}
	}
}
