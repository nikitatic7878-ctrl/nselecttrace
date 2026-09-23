// Package connections turns the socket listing produced by internal/ports into a
// view of active network connections.
//
// It deliberately owns no data source of its own. A connection is simply a
// socket that has a remote endpoint, so the package is a filter and a renderer
// over []ports.Port:
//
//	internal/ports  ->  []ports.Port  ->  internal/connections  ->  cli
//
// This is what keeps `nselecttrace ports` and `nselecttrace connections` from drifting
// apart: both read the same table through the same parser, and only the
// selection and the column layout differ.
package connections

import (
	"fmt"
	"sort"

	"nselecttrace/internal/ports"
)

// List returns the active connections of the local machine: the sockets that
// have a remote endpoint, ordered by protocol, local endpoint, remote endpoint,
// state and PID.
//
// The listing comes from ports.List, so no socket table is read twice and no
// second process cache is created.
func List() ([]ports.Port, error) {
	all, err := ports.List()
	if err != nil {
		return nil, err
	}
	return Select(all), nil
}

// Select returns the connections among ps, filtered and sorted.
//
// A socket counts as a connection when it has a remote endpoint. Listening and
// plainly bound sockets are excluded, which is the whole difference between
// this command and `nselecttrace ports`.
func Select(ps []ports.Port) []ports.Port {
	return Sort(OnlyWithRemote(ps))
}

// OnlyWithRemote returns the sockets of ps that have a remote endpoint. The
// input is not modified.
func OnlyWithRemote(ps []ports.Port) []ports.Port {
	out := make([]ports.Port, 0, len(ps))
	for _, p := range ps {
		if !p.HasRemote() {
			continue
		}
		// A listener has no remote endpoint, so this is redundant on a
		// well-behaved platform; it guards against a platform that reports a
		// listening socket with a stray remote port.
		if p.State == ports.StateListen {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Filter narrows a connection listing. The zero value matches everything.
//
// Embedded ports.Filter is the same type used by `nselecttrace ports`, so --tcp,
// --port, --pid and --process mean exactly the same thing in both commands.
// RemoteOnly is ignored here: every connection already has a remote endpoint.
type Filter struct {
	ports.Filter

	// RemotePort keeps only connections to this remote port, which is how one
	// asks for "everything talking to 443".
	RemotePort uint16
}

// IsZero reports whether the filter matches every connection.
func (f Filter) IsZero() bool {
	return f.Filter.IsZero() && f.RemotePort == 0
}

// Match reports whether p satisfies every condition that is set.
func (f Filter) Match(p ports.Port) bool {
	if !f.Filter.Match(p) {
		return false
	}
	if f.RemotePort != 0 && p.RemotePort != f.RemotePort {
		return false
	}
	return true
}

// Apply returns the connections of ps that satisfy f. The input is not modified.
func Apply(ps []ports.Port, f Filter) []ports.Port {
	if f.IsZero() {
		return ps
	}

	out := make([]ports.Port, 0, len(ps))
	for _, p := range ps {
		if f.Match(p) {
			out = append(out, p)
		}
	}
	return out
}

// Sort orders ps deterministically: protocol, local endpoint, remote endpoint,
// state, PID.
//
// The platform reports sockets in whatever order its table happens to have, so
// nothing here may rely on it.
func Sort(ps []ports.Port) []ports.Port {
	sorted := make([]ports.Port, len(ps))
	copy(sorted, ps)
	sort.SliceStable(sorted, func(i, j int) bool {
		return less(sorted[i], sorted[j])
	})
	return sorted
}

func less(a, b ports.Port) bool {
	if a.Protocol != b.Protocol {
		return a.Protocol < b.Protocol
	}
	if a.LocalIP != b.LocalIP {
		return a.LocalIP < b.LocalIP
	}
	if a.LocalPort != b.LocalPort {
		return a.LocalPort < b.LocalPort
	}
	if a.RemoteIP != b.RemoteIP {
		return a.RemoteIP < b.RemoteIP
	}
	if a.RemotePort != b.RemotePort {
		return a.RemotePort < b.RemotePort
	}
	if a.State != b.State {
		return a.State < b.State
	}
	return a.PID < b.PID
}

// CountUnknownProcesses reports how many connections have no resolved process
// name, so the CLI can explain the same privilege limitation as `nselecttrace ports`.
func CountUnknownProcesses(ps []ports.Port) int {
	return ports.CountUnknownProcesses(ps)
}

// String implements fmt.Stringer for convenient logging in tests.
func (f Filter) String() string {
	return fmt.Sprintf("Filter{protocol=%v localPort=%d pid=%d process=%q state=%v remotePort=%d}",
		f.Protocol, f.LocalPort, f.PID, f.ProcessSub, f.State, f.RemotePort)
}
