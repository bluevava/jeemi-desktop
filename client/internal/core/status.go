package core

// State is the lifecycle state of the external mihomo process.
type State string

const (
	StateNotInstalled State = "not_installed"
	StateDownloading  State = "downloading"
	StateReady        State = "ready"
	StateStarting     State = "starting"
	StateRunning      State = "running"
	StateStopping     State = "stopping"
	StateStopped      State = "stopped"
	StateFailed       State = "failed"
)

type Status struct {
	State           State  `json:"state"`
	Version         string `json:"version"`
	ControllerReady bool   `json:"controllerReady"`
}

func InitialStatus() Status {
	return Status{State: StateNotInstalled}
}
