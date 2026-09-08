package processmemory

import "math"

// Snapshot reports the process memory that belongs to Jeemi's desktop
// process plus its authorization helper, its embedded web renderer, and the
// managed mihomo process tree. Windows uses private working set (private commit
// on older systems). Linux/macOS
// uses RSS, including shared pages, because file-capability cores do not allow
// the ordinary GUI to read smaps. A nil value means the measurement is unavailable.
type Snapshot struct {
	ClientBytes  *uint64 `json:"clientBytes"`
	WebViewBytes *uint64 `json:"webViewBytes"`
	MihomoBytes  *uint64 `json:"mihomoBytes"`
	TotalBytes   *uint64 `json:"totalBytes"`
}

// CaptureDesktop reads the ordinary GUI and its web processes. The core PID
// only excludes that tree from the GUI group; privileged memory comes over IPC.
func CaptureDesktop(mihomoPID int) Snapshot {
	return captureDesktop(mihomoPID)
}

// HelperSnapshot is private IPC data, never a Wails process-query capability.
// MihomoPID binds the measurement to the sampled managed process.
type HelperSnapshot struct {
	HelperBytes  *uint64 `json:"helperBytes"`
	MihomoBytes  *uint64 `json:"mihomoBytes"`
	MihomoPID    int     `json:"mihomoPID"`
	WebViewBytes *uint64 `json:"webViewBytes,omitempty"`
}

// AbsentHelper is used only after confirming that no helper is installed.
func AbsentHelper() HelperSnapshot {
	return HelperSnapshot{HelperBytes: valuePointer(0), MihomoBytes: valuePointer(0)}
}

// Combine never substitutes a GUI query or stale measurement for helper data.
func Combine(desktop Snapshot, helper HelperSnapshot, mihomoPID int) Snapshot {
	core := helper.MihomoBytes
	if helper.MihomoPID != mihomoPID || mihomoPID < 0 {
		core = nil
	}
	web := desktop.WebViewBytes
	if web == nil {
		web = helper.WebViewBytes
	}
	return buildSnapshot(sum(desktop.ClientBytes, helper.HelperBytes), web, core)
}

func sum(values ...*uint64) *uint64 {
	var total uint64
	for _, value := range values {
		if value == nil || *value > math.MaxUint64-total {
			return nil
		}
		total += *value
	}
	return valuePointer(total)
}

func buildSnapshot(client, webView, mihomo *uint64) Snapshot {
	result := Snapshot{
		ClientBytes:  client,
		WebViewBytes: webView,
		MihomoBytes:  mihomo,
	}
	result.TotalBytes = sum(client, webView, mihomo)
	return result
}

func valuePointer(value uint64) *uint64 {
	return &value
}
