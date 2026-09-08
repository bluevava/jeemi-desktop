package processmemory

import "strings"

// Kept independent of cgo so ownership and incomplete-sample handling can be
// tested on every host. Memory is read only after establishing membership.
type darwinProcess struct {
	pid, parent        int
	uid                uint32
	started, coalition uint64
	name               string
}

func darwinWebProcess(p darwinProcess) bool {
	return strings.HasPrefix(p.name, "com.apple.WebKit.") || strings.HasPrefix(p.name, "WebKit")
}

func darwinMembers(processes map[int]darwinProcess, rootPID, excludedPID int) map[int]bool {
	members := map[int]bool{}
	root, ok := processes[rootPID]
	if !ok || rootPID <= 0 {
		return members
	}
	queue := []darwinProcess{root}
	children := map[int][]darwinProcess{}
	for _, p := range processes {
		children[p.parent] = append(children[p.parent], p)
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if members[p.pid] || p.pid == excludedPID {
			continue
		}
		members[p.pid] = true
		for _, child := range children[p.pid] {
			if child.started >= p.started {
				queue = append(queue, child)
			}
		}
	}
	return members
}

func darwinDesktop(processes map[int]darwinProcess, rootPID, corePID int, read func(darwinProcess) *uint64) Snapshot {
	root, ok := processes[rootPID]
	if !ok {
		return Snapshot{}
	}
	members := darwinMembers(processes, rootPID, corePID)
	web := map[int]bool{}
	parent, parentKnown := processes[root.parent]
	// launchd may parent WebKit XPC services directly. Only use a resource
	// coalition distinct from the GUI's parent (never the shell's shared group).
	attributed := root.coalition != 0 && parentKnown && parent.coalition != 0 && root.coalition != parent.coalition
	for _, p := range processes {
		if p.pid == rootPID || !darwinWebProcess(p) {
			continue
		}
		owned := members[p.pid]
		if p.uid == root.uid && p.started >= root.started {
			if p.coalition == 0 {
				attributed = false
			}
			owned = owned || (attributed && p.coalition == root.coalition)
		}
		if !owned {
			continue
		}
		for pid := range darwinMembers(processes, p.pid, corePID) {
			web[pid] = true
		}
	}
	clientBytes, webBytes := read(root), valuePointer(0)
	if !attributed {
		webBytes = nil
	} else {
		for pid := range web {
			webBytes = sum(webBytes, read(processes[pid]))
		}
	}
	return buildSnapshot(clientBytes, webBytes, nil)
}
