//go:build darwin && !jeemi_local_test

package macnetwork

func localManage(string) error      { return Failure("invalid_request") }
func InstallLocalFromBundle() error { return Failure("invalid_request") }
