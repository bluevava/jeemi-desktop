//go:build (!windows && !darwin && !linux) || (darwin && !cgo)

package sessionend

func start(_ *request, _ func()) (func(), error) { return func() {}, nil }
