// Package helperstate separates persistent installation from IPC availability.
package helperstate

const Protocol = 2 // Linux service supports scoped process memory queries.

type Facts struct {
	Present    bool
	Registered bool
	Trusted    bool
	Reachable  bool
	Protocol   int
	Digest     string
}

type State struct {
	Present bool
	Ready   bool
	Code    string
	Action  string
}

func Evaluate(f Facts, expected string) State {
	return EvaluateProtocol(f, expected, Protocol)
}

func EvaluateProtocol(f Facts, expected string, protocol int) State {
	s := State{Present: f.Present || f.Registered, Code: "authorization_install_required", Action: "install"}
	if !s.Present {
		return s
	}
	s.Code, s.Action = "authorization_repair_required", "repair"
	if !f.Registered || !f.Trusted {
		return s
	}
	if len(expected) != 64 {
		s.Code, s.Action = "authorization_bundle_required", ""
		return s
	}
	if f.Digest != expected || (f.Reachable && f.Protocol != protocol) {
		s.Code, s.Action = "authorization_update_required", "update"
		return s
	}
	if !f.Reachable {
		return s
	}
	s.Ready, s.Code, s.Action = true, "ready", ""
	return s
}
