package appupdate

import "golang.org/x/sys/windows"

// Inspect the flags actually received by the test subprocess. No window is
// created, but SW_HIDE would suppress a real GUI's initial ShowWindow call.
func updateProcessPresentationOK(hidden bool) bool {
	var info windows.StartupInfo
	if windows.GetStartupInfo(&info) != nil {
		return false
	}
	if hidden {
		return info.Flags&windows.STARTF_USESHOWWINDOW != 0 && info.ShowWindow == windows.SW_HIDE
	}
	return info.Flags&windows.STARTF_USESHOWWINDOW == 0
}
