//go:build !darwin || !jeemi_local_test

package desktop

func runLocalNetworkStatus() bool { return false }
