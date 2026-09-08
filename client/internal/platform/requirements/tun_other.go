//go:build !linux

package requirements

func CheckTUN(string, int) error { return nil }

func CheckCoreCapabilities(string, int) error { return nil }
