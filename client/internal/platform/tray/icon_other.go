//go:build !linux

package tray

func prepareTrayIcon(icon []byte) ([]byte, error) { return icon, nil }
