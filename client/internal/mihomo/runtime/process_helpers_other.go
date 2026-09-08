//go:build !linux

package runtime

import "os"

func retainProcessThread() func()                     { return func() {} }
func forceTerminateProcess(process *os.Process) error { return process.Kill() }
