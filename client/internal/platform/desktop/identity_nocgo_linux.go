//go:build linux && !cgo && !bindings

package desktop

// Headless builds can test registration without GTK; the Wails GUI requires cgo.
func prepareNativeIdentity() {}
