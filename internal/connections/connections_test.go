package connections

import (
	"testing"

	"nselecttrace/internal/ports"
)

// fixture is a synthetic socket listing covering every selection case the spec
// calls out, plus the endpoints needed to check ordering and IPv6 formatting.
//
// It is a fixture rather than a live reading so that no test depends on what
// happens to be connected on the machine running it.
func fixture() []ports.Port {
	return []ports.Port{
		// Included: established, both IPv4 endpoints.
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 52341,
			RemoteIP: "142.250.74.14", RemotePort: 443,
			State: ports.StateEstablished, PID: 8416, Process: "chrome.exe"},
		// Included: second connection of the same process.
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 52342,
			RemoteIP: "1.1.1.1", RemotePort: 443,
			State: ports.StateEstablished, PID: 8416, Process: "chrome.exe"},
		// Included: loopback connection.
		{Protocol: ports.TCP, LocalIP: "127.0.0.1", LocalPort: 52143,
			RemoteIP: "127.0.0.1", RemotePort: 3000,
			State: ports.StateEstablished, PID: 8212, Process: "node.exe"},
		// Included: TIME_WAIT, and its owner is already gone.
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 53122,
			RemoteIP: "192.168.1.10", RemotePort: 22,
			State: ports.StateTimeWait, PID: 0, Process: ""},
		// Included: CLOSE_WAIT.
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 53123,
			RemoteIP: "192.168.1.11", RemotePort: 80,
			State: ports.StateCloseWait, PID: 9144, Process: "postgres.exe"},
		// Included: SYN_SENT.
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 53124,
			RemoteIP: "192.168.1.12", RemotePort: 8080,
			State: ports.StateSynSent, PID: 8416, Process: "chrome.exe"},
		// Included: IPv6 connection, which must be bracketed when rendered.
		{Protocol: ports.TCP, LocalIP: "fe80::1234", LocalPort: 53126,
			RemoteIP: "2607:f8b0::1", RemotePort: 443,
			State: ports.StateEstablished, PID: 8416, Process: "chrome.exe"},
		// Excluded: a listener has no remote endpoint.
		{Protocol: ports.TCP, LocalIP: "0.0.0.0", LocalPort: 3000,
			State: ports.StateListen, PID: 8212, Process: "node.exe"},
		// Excluded: loopback listener.
		{Protocol: ports.TCP, LocalIP: "127.0.0.1", LocalPort: 5432,
			State: ports.StateListen, PID: 9144, Process: "postgres.exe"},
		// Excluded: bound UDP socket with no remote endpoint.
		{Protocol: ports.UDP, LocalIP: "0.0.0.0", LocalPort: 53,
			State: ports.StateNone, PID: 1400, Process: "dns.exe"},
		// Excluded: IPv6 bound UDP socket with no remote endpoint.
		{Protocol: ports.UDP, LocalIP: "::", LocalPort: 5353,
			State: ports.StateNone, PID: 200, Process: "mdns.exe"},
	}
}

// connectionPorts returns just the local ports of a listing, for assertions that
// care about membership rather than column layout.
func connectionPorts(ps []ports.Port) []uint16 {
	out := make([]uint16, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.LocalPort)
	}
	return out
}

func hasPort(ps []ports.Port, port uint16) bool {
	for _, p := range ps {
		if p.LocalPort == port {
			return true
		}
	}
	return false
}

func TestOnlyWithRemoteExcludesListenersAndBoundSockets(t *testing.T) {
	got := OnlyWithRemote(fixture())

	// Every socket that must be excluded.
	for _, excluded := range []uint16{3000, 5432, 53, 5353} {
		if hasPort(got, excluded) {
			t.Errorf("port %d was not excluded:\n%v", excluded, connectionPorts(got))
		}
	}

	// Every socket that must be included.
	for _, included := range []uint16{52341, 52342, 52143, 53122, 53123, 53124, 53126} {
		if !hasPort(got, included) {
			t.Errorf("port %d is missing:\n%v", included, connectionPorts(got))
		}
	}

	if len(got) != 7 {
		t.Errorf("len = %d, want 7:\n%v", len(got), connectionPorts(got))
	}
}

func TestOnlyWithRemoteKeepsEveryTcpState(t *testing.T) {
	// TIME_WAIT, CLOSE_WAIT and SYN_SENT are connections, not noise.
	states := map[ports.State]bool{}
	for _, p := range OnlyWithRemote(fixture()) {
		states[p.State] = true
	}
	for _, want := range []ports.State{
		ports.StateEstablished, ports.StateTimeWait, ports.StateCloseWait, ports.StateSynSent,
	} {
		if !states[want] {
			t.Errorf("state %v was dropped from the listing", want)
		}
	}
}

func TestOnlyWithRemoteExcludesListenerEvenWithStrayRemotePort(t *testing.T) {
	// A platform that reports a listening socket with a remote port set must not
	// let it through: LISTENING means there is no peer.
	ps := []ports.Port{
		{Protocol: ports.TCP, LocalIP: "0.0.0.0", LocalPort: 3000,
			RemoteIP: "10.0.0.1", RemotePort: 1234,
			State: ports.StateListen, PID: 1, Process: "x"},
	}

	if got := OnlyWithRemote(ps); len(got) != 0 {
		t.Errorf("a LISTENING socket leaked into connections: %+v", got)
	}
}

func TestSelectDoesNotMutateInput(t *testing.T) {
	all := fixture()
	before := len(all)
	first := all[0]

	_ = Select(all)

	if len(all) != before {
		t.Errorf("Select changed the input length: %d, want %d", len(all), before)
	}
	if all[0] != first {
		t.Errorf("Select reordered the input: %+v", all[0])
	}
}
