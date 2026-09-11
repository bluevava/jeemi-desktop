package updateprocess

import "os/exec"

// Start launches a hidden ordinary-user update worker without a console or shell.
func Start(command *exec.Cmd) error {
	configure(command, true)
	return command.Start()
}

// StartGUI restarts the client without a console or shell, while allowing its
// main window to be shown normally.
func StartGUI(command *exec.Cmd) error {
	configure(command, false)
	return command.Start()
}
