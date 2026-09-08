package processmemory

import "testing"

func TestDarwinWebKitUsesOwnTreeOrIsolatedCoalition(t *testing.T) {
	processes := map[int]darwinProcess{
		1:  {pid: 1, started: 1, coalition: 1},
		10: {pid: 10, parent: 1, uid: 501, started: 10, coalition: 5, name: "Jeemi"},
		11: {pid: 11, parent: 10, uid: 501, started: 11, coalition: 5, name: "com.apple.WebKit.WebContent"},
		12: {pid: 12, parent: 1, uid: 501, started: 12, coalition: 5, name: "com.apple.WebKit.Networking"},
		13: {pid: 13, parent: 12, uid: 501, started: 13, coalition: 5, name: "worker"},
		14: {pid: 14, parent: 10, uid: 501, started: 14, coalition: 5, name: "support"},
		15: {pid: 15, parent: 10, uid: 501, started: 5, coalition: 5, name: "WebKitOldPID"},
		20: {pid: 20, parent: 1, uid: 501, started: 20, coalition: 6, name: "com.apple.WebKit.WebContent"},
		30: {pid: 30, parent: 1, uid: 502, started: 30, coalition: 5, name: "com.apple.WebKit.Networking"},
	}
	seen := map[int]bool{}
	result := darwinDesktop(processes, 10, 0, func(p darwinProcess) *uint64 { seen[p.pid] = true; return valuePointer(uint64(p.pid)) })
	if result.ClientBytes == nil || *result.ClientBytes != 10 || result.WebViewBytes == nil || *result.WebViewBytes != 36 {
		t.Fatalf("bad grouping %+v", result)
	}
	for _, pid := range []int{1, 15, 20, 30} {
		if seen[pid] {
			t.Fatalf("queried foreign process %d", pid)
		}
	}
}

func TestDarwinSharedOrUnknownCoalitionDoesNotInventWebTotal(t *testing.T) {
	for _, coalition := range []uint64{0, 1} {
		processes := map[int]darwinProcess{
			1:  {pid: 1, coalition: 1},
			10: {pid: 10, parent: 1, uid: 501, started: 10, coalition: coalition},
			11: {pid: 11, parent: 1, uid: 501, started: 11, coalition: coalition, name: "com.apple.WebKit.Networking"},
		}
		result := darwinDesktop(processes, 10, 0, func(p darwinProcess) *uint64 {
			if p.pid != 10 {
				t.Fatal("shared coalition was treated as owned")
			}
			return valuePointer(10)
		})
		if result.WebViewBytes != nil || result.ClientBytes == nil {
			t.Fatal("uncertain web attribution should not mask self memory or invent a total")
		}
	}
}
