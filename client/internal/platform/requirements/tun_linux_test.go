//go:build linux

package requirements

import (
	"encoding/binary"
	"testing"
)

func TestTUNCapabilityInheritance(t *testing.T) {
	caps := make([]byte, 20)
	binary.LittleEndian.PutUint32(caps, 0x02000001)
	binary.LittleEndian.PutUint32(caps[4:], uint32(tunCapabilities))
	for _, test := range []struct {
		name, status string
		root, nosuid bool
		caps         []byte
		want         bool
	}{
		{"ordinary user", "CapBnd:\tffff\n", false, false, nil, false},
		{"authorized file", "CapBnd:\tffff\n", false, false, caps, true},
		{"no new privileges", "CapBnd:\tffff\nNoNewPrivs:\t1\n", false, false, caps, false},
		{"nosuid mount", "CapBnd:\tffff\n", false, true, caps, false},
		{"container bounding limit", "CapBnd:\t0000\n", false, false, caps, false},
		{"ambient permissions", "CapAmb:\t3400\n", false, false, nil, true},
		{"old grant misses low ports", "CapAmb:\t3000\nCapBnd:\tffff\n", false, false, nil, false},
		{"nonroot effective caps do not survive exec", "CapBnd:\tffff\nCapEff:\t3400\n", false, false, nil, false},
		{"capable root", "CapBnd:\tffff\nCapEff:\t3400\n", true, false, nil, true},
		{"truncated xattr", "CapBnd:\tffff\n", false, false, caps[:10], false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := canStartTUN(test.status, test.caps, test.root, test.nosuid); got != test.want {
				t.Fatalf("canStartTUN = %v", got)
			}
		})
	}
}
