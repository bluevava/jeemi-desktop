//go:build !linux || bindings

package desktop

func Prepare(_ []byte) error { return nil }
