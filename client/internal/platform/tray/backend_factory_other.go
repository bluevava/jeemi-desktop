//go:build !darwin && !linux

package tray

func newSystemBackend() backend { return systemTrayBackend{} }
