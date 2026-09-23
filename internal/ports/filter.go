package ports

import (
	"strings"
)

// Filter narrows a port listing. The zero value matches everything.
//
// Filtering is applied to the domain model, never inside the platform source:
// the operating system reports every socket and nselecttrace decides what to show.
// `nselecttrace ports` and `nselecttrace connections` share this type so that a flag means
// the same thing in both commands.
type Filter struct {
	Protocol   Protocol // 0 means any protocol
	LocalPort  uint16   // 0 means any port
	PID        uint32   // 0 means any process
	ProcessSub string   // empty means any process; matched case-insensitively
	State      State    // StateNone means any state

	// RemoteOnly keeps only sockets that have a remote endpoint, which is what
	// makes a socket a connection. StateListen sockets are dropped as well:
	// a listener has no remote endpoint by definition, but a platform that
	// reports a port without a state would otherwise leak one through.
	RemoteOnly bool
}

// IsZero reports whether the filter matches every port.
func (f Filter) IsZero() bool {
	return f.Protocol == 0 && f.LocalPort == 0 && f.PID == 0 && f.ProcessSub == "" &&
		f.State == StateNone && !f.RemoteOnly
}

// Match reports whether p satisfies every condition that is set.
func (f Filter) Match(p Port) bool {
	if f.Protocol != 0 && p.Protocol != f.Protocol {
		return false
	}
	if f.LocalPort != 0 && p.LocalPort != f.LocalPort {
		return false
	}
	if f.PID != 0 && p.PID != f.PID {
		return false
	}
	if f.ProcessSub != "" && !strings.Contains(strings.ToLower(p.Process), strings.ToLower(f.ProcessSub)) {
		return false
	}
	// A State filter cannot be satisfied by a protocol without a state
	// machine, so UDP never matches --state ESTABLISHED. That is deliberate:
	// inventing a state for UDP would be worse than not matching.
	if f.State != StateNone && p.State != f.State {
		return false
	}
	if f.RemoteOnly && !p.HasRemote() {
		return false
	}
	return true
}

// Apply returns the ports of ps that satisfy f. The input is not modified.
func Apply(ps []Port, f Filter) []Port {
	if f.IsZero() {
		return ps
	}

	out := make([]Port, 0, len(ps))
	for _, p := range ps {
		if f.Match(p) {
			out = append(out, p)
		}
	}
	return out
}

// CountUnknownProcesses reports how many ports have no resolved process name.
//
// The CLI uses it to explain that process attribution is best effort: reading
// another user's process requires privileges on Windows and macOS, so an empty
// name is expected rather than a bug.
func CountUnknownProcesses(ps []Port) int {
	n := 0
	for _, p := range ps {
		if p.Process == "" {
			n++
		}
	}
	return n
}
