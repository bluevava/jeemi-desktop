//go:build !linux

package processguard

func RunIfRequested() (bool, error) { return false, nil }
