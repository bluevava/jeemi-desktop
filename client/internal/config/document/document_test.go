package document

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseRejectsDuplicateKeysAndMultipleDocuments(t *testing.T) {
	cases := []string{
		"dns:\n  enable: true\n  enable: false\n",
		"mode: rule\n---\nmode: direct\n",
	}
	for _, input := range cases {
		if _, err := Parse([]byte(input)); err == nil {
			t.Fatalf("Parse(%q) returned no error", input)
		}
	}
}

func TestParseEnforcesSizeDepthAliasAndTagLimits(t *testing.T) {
	if _, err := parse(bytes.Repeat([]byte{'a'}, 33), Limits{MaxBytes: 32, MaxNodes: 100, MaxDepth: 10, MaxAliases: 2}); err == nil {
		t.Fatal("parse() accepted an oversized document")
	}
	var deepBuilder strings.Builder
	for level := 0; level < 8; level++ {
		deepBuilder.WriteString(strings.Repeat("  ", level))
		deepBuilder.WriteString("child:\n")
	}
	deepBuilder.WriteString(strings.Repeat("  ", 8))
	deepBuilder.WriteString("value: true\n")
	deep := deepBuilder.String()
	if _, err := parse([]byte(deep), Limits{MaxBytes: 1024, MaxNodes: 100, MaxDepth: 5, MaxAliases: 2}); err == nil {
		t.Fatal("parse() accepted an over-deep document")
	}
	aliases := "base: &base\n  enabled: true\ncopy: *base\n"
	if _, err := parse([]byte(aliases), Limits{MaxBytes: 1024, MaxNodes: 100, MaxDepth: 10, MaxAliases: 0}); err == nil {
		t.Fatal("parse() accepted aliases above the configured limit")
	}
	if _, err := Parse([]byte("value: !unsafe secret\n")); err == nil {
		t.Fatal("Parse() accepted a custom YAML tag")
	}
}

func TestParsePreservesUnknownFieldsAndSetMappingPath(t *testing.T) {
	document, err := Parse([]byte("future-feature:\n  enabled: true\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	value, err := ParseValue("\n- 1.1.1.1\n- tls://dns.example\n")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	if err := SetMappingPath(Root(document), "/dns/nameserver", value); err != nil {
		t.Fatalf("SetMappingPath() error = %v", err)
	}
	encoded, err := Encode(document)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	text := string(encoded)
	for _, expected := range []string{"future-feature:", "dns:", "nameserver:", "tls://dns.example"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("encoded YAML missing %q:\n%s", expected, text)
		}
	}
}

func TestPointerSegmentsHandlesEscapes(t *testing.T) {
	segments, err := PointerSegments("/nameserver-policy/+.~1example~0test")
	if err != nil {
		t.Fatalf("PointerSegments() error = %v", err)
	}
	if got, want := strings.Join(segments, "|"), "nameserver-policy|+./example~test"; got != want {
		t.Fatalf("segments = %q, want %q", got, want)
	}
}

func TestDeleteMappingPathRemovesValueAndPrunesEmptyParents(t *testing.T) {
	configuration, err := Parse([]byte("mode: rule\ntun:\n  enable: true\nother: kept\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteMappingPath(Root(configuration), "/tun/enable"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := Find(Root(configuration), "/tun"); err != nil || found {
		t.Fatalf("empty parent was not pruned: found=%v err=%v", found, err)
	}
	if err := DeleteMappingPath(Root(configuration), "/missing/path"); err != nil {
		t.Fatalf("missing path was not idempotent: %v", err)
	}
	if value, found, err := Find(Root(configuration), "/other"); err != nil || !found || value.Value != "kept" {
		t.Fatalf("unrelated value changed: value=%+v found=%v err=%v", value, found, err)
	}
}
