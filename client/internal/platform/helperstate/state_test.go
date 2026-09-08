package helperstate

import "testing"

func TestInstallationIsIndependentOfLiveness(t *testing.T) {
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, test := range []struct {
		name   string
		facts  Facts
		action string
		ready  bool
	}{
		{"absent", Facts{}, "install", false},
		{"stopped", Facts{Registered: true, Trusted: true, Digest: digest}, "repair", false},
		{"partial", Facts{Present: true}, "repair", false},
		{"untrusted process", Facts{Registered: true, Reachable: true, Digest: digest, Protocol: Protocol}, "repair", false},
		{"old binary", Facts{Registered: true, Trusted: true, Reachable: true, Protocol: Protocol, Digest: "old"}, "update", false},
		{"old protocol", Facts{Registered: true, Trusted: true, Reachable: true, Protocol: 99, Digest: digest}, "update", false},
		{"healthy", Facts{Registered: true, Trusted: true, Reachable: true, Protocol: Protocol, Digest: digest}, "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := Evaluate(test.facts, digest)
			if s.Action != test.action || s.Ready != test.ready {
				t.Fatalf("unexpected state: %+v", s)
			}
		})
	}
}
