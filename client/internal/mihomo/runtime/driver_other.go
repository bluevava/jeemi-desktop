//go:build !darwin && !windows

package runtime

func platformDriver() Driver { return commandDriver{} }
