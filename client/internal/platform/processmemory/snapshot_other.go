//go:build !windows && !linux && (!darwin || !cgo)

package processmemory

func captureDesktop(mihomoPID int) Snapshot {
	mihomo := (*uint64)(nil)
	if mihomoPID <= 0 {
		mihomo = valuePointer(0)
	}
	return buildSnapshot(nil, nil, mihomo)
}

func CaptureHelper(mihomoPID int) HelperSnapshot {
	result := HelperSnapshot{MihomoPID: mihomoPID}
	if mihomoPID <= 0 {
		result.MihomoBytes = valuePointer(0)
	}
	return result
}

func CaptureWebView(clientPID, mihomoPID int) *uint64 { return nil }
