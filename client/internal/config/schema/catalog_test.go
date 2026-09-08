package schema

import (
	"testing"

	"jeemi/internal/config/document"
	"jeemi/internal/config/geodataoverride"
	"jeemi/internal/config/runtimeoverride"

	"gopkg.in/yaml.v3"
)

func TestCatalogHasUniqueValidFieldsAndExamples(t *testing.T) {
	seenIDs := make(map[string]struct{})
	seenPaths := make(map[string]struct{})
	for _, category := range Current().Categories {
		if category.ID == "" || category.DocumentationURL == "" {
			t.Fatalf("invalid category: %+v", category)
		}
		for _, field := range category.Fields {
			if _, duplicate := seenIDs[field.ID]; duplicate {
				t.Fatalf("duplicate field id %q", field.ID)
			}
			seenIDs[field.ID] = struct{}{}
			if _, duplicate := seenPaths[field.Path]; duplicate {
				t.Fatalf("duplicate field path %q", field.Path)
			}
			seenPaths[field.Path] = struct{}{}
			if _, err := document.PointerSegments(field.Path); err != nil {
				t.Fatalf("invalid path %q: %v", field.Path, err)
			}
			if field.Locked {
				if len(field.Strategies) != 0 {
					t.Fatalf("locked field %q exposes strategies", field.Path)
				}
				continue
			}
			value, err := document.ParseValue(field.ExampleYAML)
			if err != nil {
				t.Fatalf("invalid example for %q: %v", field.Path, err)
			}
			if !catalogKindMatches(field.Kind, value.Kind) {
				t.Fatalf("example for %q has YAML kind %d, want %s", field.Path, value.Kind, field.Kind)
			}
			if len(field.Strategies) == 0 || field.DefaultStrategy == "" {
				t.Fatalf("editable field %q has no strategy", field.Path)
			}
		}
	}
}

func catalogKindMatches(expected ValueKind, actual yaml.Kind) bool {
	switch expected {
	case KindScalar:
		return actual == yaml.ScalarNode
	case KindMapping, KindNamedMapping:
		return actual == yaml.MappingNode
	case KindSequence, KindNamedSequence, KindRules:
		return actual == yaml.SequenceNode
	default:
		return false
	}
}

func TestCatalogLocksRuntimeOwnedFields(t *testing.T) {
	paths := []string{"/external-controller", "/secret", "/external-controller-cors"}
	paths = append(paths, runtimeoverride.ManagedPaths()...)
	paths = append(paths, geodataoverride.ManagedPaths()...)
	for _, path := range paths {
		field, found := FindField(path)
		if !found || !field.Locked {
			t.Fatalf("runtime-owned field %q is not locked", path)
		}
	}
}

func TestCatalogExposesStructuredSnifferAndProxyTransforms(t *testing.T) {
	for _, path := range []string{
		"/sniffer/enable",
		"/sniffer/sniff/HTTP/ports",
		"/sniffer/sniff/TLS/override-destination",
		"/sniffer/skip-domain",
	} {
		field, found := FindField(path)
		if !found || field.Hidden || field.Scope != ScopeDirect {
			t.Fatalf("structured sniffer field %q is unavailable: %+v", path, field)
		}
	}
	if field, found := FindField("/sniffer"); found {
		t.Fatalf("first-release catalog retained an obsolete aggregate field: %+v", field)
	}
	fingerprint, found := FindField("/proxies/*/client-fingerprint")
	if !found || fingerprint.Scope != ScopeAllProxies || fingerprint.TargetPath != "/client-fingerprint" {
		t.Fatalf("proxy fingerprint transform metadata is incomplete: %+v", fingerprint)
	}
	if len(fingerprint.ApplicableTypes) != 4 {
		t.Fatalf("proxy fingerprint compatibility list is incomplete: %+v", fingerprint.ApplicableTypes)
	}
}
