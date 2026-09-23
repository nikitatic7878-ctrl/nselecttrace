// Package ports enumerates the network sockets of the local machine.
//
// The package returns a platform-neutral domain model: the CLI, a future TUI
// and JSON output all consume the same values, and no operating system handle
// or table layout leaks out of the package.
package ports

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"

	"nselecttrace/internal/processes"
)

// Protocol is the transport protocol of a socket.
type Protocol uint8

// Transport protocols. Only protocols that can actually be enumerated are
// declared here.
const (
	TCP Protocol = iota + 1
	UDP
)

// String implements fmt.Stringer.
func (p Protocol) String() string {
	switch p {
	case TCP:
		return "TCP"
	case UDP:
		return "UDP"
	default:
		return fmt.Sprintf("PROTO(%d)", uint8(p))
	}
}

// State is the connection state of a TCP socket.
//
// UDP sockets have no state machine, so a UDP port always carries StateNone.
// StateNone must not be confused with StateUnknown: the first means the
// protocol has no state, the second means the operating system reported a state
// this package does not know about.
type State uint8

// Socket states.
const (
	StateNone State = iota
	StateClosed
	StateListen
	StateSynSent
	StateSynReceived
	StateEstablished
	StateFinWait1
	StateFinWait2
	StateCloseWait
	StateClosing
	StateLastAck
	StateTimeWait
	StateDeleteTCB
	StateUnknown
)

// String implements fmt.Stringer, using the conventional names from the TCP
// state machine so that the output is familiar to anyone who has read
// `netstat` or `ss`.
func (s State) String() string {
	switch s {
	case StateNone:
		return "-"
	case StateClosed:
		return "CLOSED"
	case StateListen:
		return "LISTENING"
	case StateSynSent:
		return "SYN_SENT"
	case StateSynReceived:
		return "SYN_RECV"
	case StateEstablished:
		return "ESTABLISHED"
	case StateFinWait1:
		return "FIN_WAIT1"
	case StateFinWait2:
		return "FIN_WAIT2"
	case StateCloseWait:
		return "CLOSE_WAIT"
	case StateClosing:
		return "CLOSING"
	case StateLastAck:
		return "LAST_ACK"
	case StateTimeWait:
		return "TIME_WAIT"
	case StateDeleteTCB:
		return "DELETE_TCB"
	case StateUnknown:
		return "UNKNOWN"
	default:
		return fmt.Sprintf("STATE(%d)", uint8(s))
	}
}

// Port is a single socket endpoint owned by a process on the local machine.
//
// A listening socket has an unspecified remote endpoint, which is represented
// by the zero value of RemoteIP and RemotePort rather than by a nil pointer:
// callers can then format the data without special cases.
type Port struct {
	Protocol   Protocol
	LocalIP    string // "" or "0.0.0.0"/"::" for a wildcard bind
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
	State      State
	PID        uint32
	Process    string // executable name; empty when it could not be resolved
}

// IsWildcard reports whether the socket is bound to every local address.
func (p Port) IsWildcard() bool {
	return p.LocalIP == "" || p.LocalIP == "0.0.0.0" || p.LocalIP == "::"
}

// HasRemote reports whether the socket has a remote endpoint.
func (p Port) HasRemote() bool {
	return p.RemotePort != 0
}

// LocalEndpoint renders the local address as "address:port".
//
// A wildcard bind is rendered as a bare ":port", which is how socket listings
// conventionally show it. IPv6 addresses are bracketed by net.JoinHostPort so
// that the colons inside the address cannot be mistaken for the port separator.
func (p Port) LocalEndpoint() string {
	if p.IsWildcard() {
		return fmt.Sprintf(":%d", p.LocalPort)
	}
	return net.JoinHostPort(p.LocalIP, strconv.Itoa(int(p.LocalPort)))
}

// RemoteEndpoint renders the remote address as "address:port", or a placeholder
// when the socket has no remote endpoint.
func (p Port) RemoteEndpoint() string {
	if !p.HasRemote() {
		return "-"
	}
	return net.JoinHostPort(p.RemoteIP, strconv.Itoa(int(p.RemotePort)))
}

// ErrUnsupported reports that socket enumeration is not implemented on the
// current platform.
var ErrUnsupported = errors.New("ports are not implemented on this platform")

// Source provides the raw socket records of the local machine.
//
// It is the only seam in this package that is platform specific. Tests supply
// their own Source, so no test depends on which sockets happen to be open on
// the machine running it.
type Source interface {
	// Records returns one record per socket. A record that cannot be
	// represented is skipped by the implementation; an error is returned only
	// when the enumeration itself failed.
	Records() ([]Record, error)
}

// Record is a single socket as reported by the operating system, with the
// process name optionally already resolved. It exists so that platform code
// never has to know about sorting, filtering or rendering.
type Record struct {
	Protocol   Protocol
	LocalIP    string
	LocalPort  uint16
	RemoteIP   string
	RemotePort uint16
	State      State
	PID        uint32
	Process    string
}

// systemSource is the platform source used by List. It is a package variable so
// that a deterministic Source can be substituted in tests.
var systemSource Source = platformSource{}

// List returns the sockets of the local machine, ordered by protocol, local
// address, port and PID. The order is stable and does not depend on the order in
// which the operating system reported the sockets.
//
// Process names are resolved through a single cache for the whole listing, and
// a socket whose owner cannot be inspected keeps an empty Process field rather
// than failing the call. Where process lookup is unavailable the sockets are
// still returned, without names.
func List() ([]Port, error) {
	return ListFrom(systemSource)
}

// ListFrom returns the sockets provided by src, sorted.
//
// It is exported so that the conversion, sorting and enrichment can be
// exercised without touching the socket tables of the running machine.
func ListFrom(src Source) ([]Port, error) {
	records, err := src.Records()
	if err != nil {
		return nil, fmt.Errorf("failed to list ports: %w", err)
	}

	cache := processes.NewCache()
	out := make([]Port, 0, len(records))
	for _, rec := range records {
		name := rec.Process
		if name == "" {
			name = cache.Lookup(rec.PID).Name
		}
		out = append(out, Port{
			Protocol:   rec.Protocol,
			LocalIP:    rec.LocalIP,
			LocalPort:  rec.LocalPort,
			RemoteIP:   rec.RemoteIP,
			RemotePort: rec.RemotePort,
			State:      rec.State,
			PID:        rec.PID,
			Process:    name,
		})
	}

	return Sort(out), nil
}

// Sort orders ps deterministically: by protocol, then local address, then local
// port, then PID.
//
// Addresses are compared textually. That keeps the result stable across runs,
// which is the property that matters here; a numeric comparison would order
// 10.0.0.1 after 9.0.0.1 and would need per-family special cases for no
// practical gain.
func Sort(ps []Port) []Port {
	sorted := make([]Port, len(ps))
	copy(sorted, ps)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.LocalIP != b.LocalIP {
			return a.LocalIP < b.LocalIP
		}
		if a.LocalPort != b.LocalPort {
			return a.LocalPort < b.LocalPort
		}
		if a.PID != b.PID {
			return a.PID < b.PID
		}
		if a.RemoteIP != b.RemoteIP {
			return a.RemoteIP < b.RemoteIP
		}
		return a.RemotePort < b.RemotePort
	})
	return sorted
}
