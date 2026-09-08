package updateprocess

import "os/exec"

// Start launches an ordinary-user process without a console or shell.
func Start(command *exec.Cmd) error { configure(command); return command.Start() }
