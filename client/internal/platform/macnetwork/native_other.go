//go:build !darwin

package macnetwork

func nativeFacts() Facts                { return Facts{} }
func nativeManage(string) error         { return Failure("unsupported") }
func nativeCall([]byte) ([]byte, error) { return nil, Failure("unsupported") }

func nativeDisconnect() {}
