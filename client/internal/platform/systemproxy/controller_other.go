//go:build !darwin && !windows

package systemproxy

func newPlatformController(path string) Controller { return newManager(path, newBackend()) }
