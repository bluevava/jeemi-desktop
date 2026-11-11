package iconcatalog

import "testing"

func TestCatalogIsUniqueAndReturnsIndependentValues(t *testing.T) {
	values := Values()
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			t.Fatal("invalid bundled Emoji catalog")
		}
		seen[value] = true
	}
	if len(values) != 150 || !seen["🧠"] || !seen["📷"] || !seen["🐱"] {
		t.Fatal("missing Emoji options")
	}
	values[0] = "changed"
	if Values()[0] == "changed" {
		t.Fatal("shared mutable catalog")
	}
}
