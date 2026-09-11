//go:build linux || darwin

package appupdate

// Unix process launches do not override a GUI's initial window visibility.
func updateProcessPresentationOK(bool) bool { return true }
