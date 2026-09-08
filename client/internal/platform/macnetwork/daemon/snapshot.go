package daemon

import (
	"io"
	"jeemi/internal/platform/coresnapshot"
)

func configurationName(name string) bool { return coresnapshot.ConfigurationName(name) }
func WriteSnapshot(root, configuration string, initial bool, output io.Writer) error {
	return coresnapshot.WriteSnapshot(root, configuration, initial, output)
}
func receiveSnapshot(directory, userRuntime string, input io.Reader, previous map[string]string) error {
	return coresnapshot.Receive(directory, userRuntime, input, previous)
}
