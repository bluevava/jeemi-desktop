package processmemory

import "testing"

func TestBuildSnapshotTotalsCompleteMeasurements(t *testing.T) {
	client := uint64(11)
	webView := uint64(22)
	mihomo := uint64(33)
	snapshot := buildSnapshot(&client, &webView, &mihomo)
	if snapshot.TotalBytes == nil || *snapshot.TotalBytes != 66 {
		t.Fatalf("TotalBytes = %v, want 66", snapshot.TotalBytes)
	}
}

func TestBuildSnapshotLeavesIncompleteTotalUnavailable(t *testing.T) {
	client := uint64(11)
	mihomo := uint64(33)
	snapshot := buildSnapshot(&client, nil, &mihomo)
	if snapshot.TotalBytes != nil {
		t.Fatalf("TotalBytes = %d, want nil", *snapshot.TotalBytes)
	}
}
