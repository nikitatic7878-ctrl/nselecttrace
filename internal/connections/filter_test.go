package connections

import (
	"testing"

	"nselecttrace/internal/ports"
)

// connections returns the fixture reduced to connections, which is what the
// filters are applied to in practice.
func connections() []ports.Port { return Select(fixture()) }

func TestFilterZeroMatchesEverything(t *testing.T) {
	all := connections()
	if !(Filter{}).IsZero() {
		t.Error("zero Filter is not reported as zero")
	}
	if got := len(Apply(all, Filter{})); got != len(all) {
		t.Errorf("zero filter matched %d connections, want all %d", got, len(all))
	}
}

func TestFilterByProtocol(t *testing.T) {
	all := connections()

	tcp := Apply(all, Filter{Filter: ports.Filter{Protocol: ports.TCP}})
	if len(tcp) != len(all) {
		t.Errorf("TCP filter matched %d, want %d", len(tcp), len(all))
	}

	// No UDP connection exists in the fixture: the two UDP sockets are bound
	// without a remote endpoint and are excluded before filtering.
	udp := Apply(all, Filter{Filter: ports.Filter{Protocol: ports.UDP}})
	if len(udp) != 0 {
		t.Errorf("UDP filter matched %d, want 0", len(udp))
	}
}

func TestFilterByUDPConnectionWithRemoteEndpoint(t *testing.T) {
	// A UDP socket that does have a remote endpoint is a connection, and it must
	// show no TCP-style state.
	ps := []ports.Port{
		{Protocol: ports.UDP, LocalIP: "192.168.1.6", LocalPort: 53000,
			RemoteIP: "8.8.8.8", RemotePort: 53,
			State: ports.StateNone, PID: 8212, Process: "app.exe"},
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 53001,
			RemoteIP: "1.1.1.1", RemotePort: 443,
			State: ports.StateEstablished, PID: 8212, Process: "app.exe"},
	}

	got := Apply(Select(ps), Filter{Filter: ports.Filter{Protocol: ports.UDP}})
	if len(got) != 1 {
		t.Fatalf("UDP filter matched %d connections, want 1", len(got))
	}
	if got[0].State != ports.StateNone {
		t.Errorf("State = %v, want StateNone for a UDP connection", got[0].State)
	}
}

func TestFilterByLocalPort(t *testing.T) {
	got := Apply(connections(), Filter{Filter: ports.Filter{LocalPort: 52341}})
	if len(got) != 1 || got[0].RemotePort != 443 {
		t.Errorf("local port filter matched %+v, want only the 52341 connection", got)
	}

	if got := Apply(connections(), Filter{Filter: ports.Filter{LocalPort: 9}}); len(got) != 0 {
		t.Errorf("unused port matched %d connections, want 0", len(got))
	}
}

func TestFilterByRemotePort(t *testing.T) {
	// Two fixture connections go to remote port 443, one of them over IPv6.
	got := Apply(connections(), Filter{RemotePort: 443})
	if len(got) != 3 {
		t.Fatalf("remote port filter matched %d connections, want 3:\n%v", len(got), connectionPorts(got))
	}
	for _, p := range got {
		if p.RemotePort != 443 {
			t.Errorf("connection %v has remote port %d, want 443", p.LocalEndpoint(), p.RemotePort)
		}
	}

	if got := Apply(connections(), Filter{RemotePort: 9}); len(got) != 0 {
		t.Errorf("unused remote port matched %d connections, want 0", len(got))
	}
}

func TestFilterByPID(t *testing.T) {
	got := Apply(connections(), Filter{Filter: ports.Filter{PID: 8212}})
	if len(got) != 1 || got[0].Process != "node.exe" {
		t.Errorf("PID filter matched %+v, want only the node connection", got)
	}

	if got := Apply(connections(), Filter{Filter: ports.Filter{PID: 999999}}); len(got) != 0 {
		t.Errorf("unknown PID matched %d connections, want 0", len(got))
	}
}

func TestFilterByProcessIsCaseInsensitive(t *testing.T) {
	for _, needle := range []string{"chrome", "CHROME", "Chrome", "chrome.exe", "ROME.EXE"} {
		got := Apply(connections(), Filter{Filter: ports.Filter{ProcessSub: needle}})
		if len(got) != 4 {
			t.Errorf("process filter %q matched %d connections, want 4", needle, len(got))
		}
	}

	if got := Apply(connections(), Filter{Filter: ports.Filter{ProcessSub: "nothing-here"}}); len(got) != 0 {
		t.Errorf("bogus process filter matched %d connections, want 0", len(got))
	}
}

func TestFilterByState(t *testing.T) {
	tests := []struct {
		state ports.State
		want  int
	}{
		{state: ports.StateEstablished, want: 4},
		{state: ports.StateTimeWait, want: 1},
		{state: ports.StateCloseWait, want: 1},
		{state: ports.StateSynSent, want: 1},
		{state: ports.StateListen, want: 0},
		{state: ports.StateFinWait1, want: 0},
	}

	for _, tt := range tests {
		got := Apply(connections(), Filter{Filter: ports.Filter{State: tt.state}})
		if len(got) != tt.want {
			t.Errorf("state %v matched %d connections, want %d", tt.state, len(got), tt.want)
			continue
		}
		for _, p := range got {
			if p.State != tt.state {
				t.Errorf("state filter %v returned a %v connection", tt.state, p.State)
			}
		}
	}
}

func TestFilterByStateNeverMatchesUDP(t *testing.T) {
	// UDP has no state machine, so asking for a TCP state must not invent one.
	ps := []ports.Port{
		{Protocol: ports.UDP, LocalIP: "10.0.0.1", LocalPort: 1, RemoteIP: "8.8.8.8", RemotePort: 53,
			State: ports.StateNone, PID: 1, Process: "a"},
	}

	got := Apply(Select(ps), Filter{Filter: ports.Filter{State: ports.StateEstablished}})
	if len(got) != 0 {
		t.Errorf("a UDP connection matched --state ESTABLISHED: %+v", got)
	}
}

func TestFilterCombinesConditions(t *testing.T) {
	// Only the node connection is loopback and owned by PID 8212.
	got := Apply(connections(), Filter{
		Filter: ports.Filter{LocalPort: 52143, PID: 8212, ProcessSub: "node"},
	})
	if len(got) != 1 {
		t.Fatalf("combined filter matched %d connections, want 1", len(got))
	}

	// Contradictory conditions must match nothing.
	if got := Apply(connections(), Filter{Filter: ports.Filter{PID: 8212}, RemotePort: 443}); len(got) != 0 {
		t.Errorf("contradictory filter matched %d connections, want 0", len(got))
	}
}

func TestFilterRemoteOnlyIsRedundantForConnections(t *testing.T) {
	// Every connection already has a remote endpoint, so setting RemoteOnly must
	// not change the result. This guards the CLI, which sets it unconditionally.
	all := connections()

	withFlag := Apply(all, Filter{Filter: ports.Filter{RemoteOnly: true}})
	if len(withFlag) != len(all) {
		t.Errorf("RemoteOnly changed the listing: %d vs %d", len(withFlag), len(all))
	}
}

func TestFilterIsZeroAccountsForRemotePort(t *testing.T) {
	if (Filter{RemotePort: 443}).IsZero() {
		t.Error("a remote port filter is reported as zero")
	}
	if (Filter{Filter: ports.Filter{State: ports.StateEstablished}}).IsZero() {
		t.Error("a state filter is reported as zero")
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	all := connections()
	before := len(all)
	first := all[0]

	_ = Apply(all, Filter{RemotePort: 443})

	if len(all) != before {
		t.Errorf("Apply changed the input length: %d, want %d", len(all), before)
	}
	if all[0] != first {
		t.Errorf("Apply reordered the input: %+v", all[0])
	}
}
