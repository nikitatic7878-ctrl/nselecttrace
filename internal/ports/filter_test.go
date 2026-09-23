package ports

import "testing"

// fixture is the port set used by the filter tests.
func fixture() []Port {
	return []Port{
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 135, State: StateListen, PID: 1200, Process: "svchost.exe"},
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 445, State: StateListen, PID: 4, Process: "System"},
		{Protocol: TCP, LocalIP: "127.0.0.1", LocalPort: 3000, State: StateListen, PID: 8212, Process: "node.exe"},
		{Protocol: TCP, LocalIP: "192.168.1.6", LocalPort: 5432, State: StateListen, PID: 9144, Process: "postgres.exe"},
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 53, State: StateNone, PID: 1400, Process: "dns.exe"},
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 5353, State: StateNone, PID: 200, Process: ""},
	}
}

func portsByLocalPort(ps []Port) []uint16 {
	out := make([]uint16, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.LocalPort)
	}
	return out
}

func TestFilterZeroMatchesEverything(t *testing.T) {
	all := fixture()
	if !(Filter{}).IsZero() {
		t.Error("zero Filter is not reported as zero")
	}
	if got := len(Apply(all, Filter{})); got != len(all) {
		t.Errorf("zero filter matched %d ports, want all %d", got, len(all))
	}
}

func TestFilterByProtocol(t *testing.T) {
	all := fixture()

	tcp := Apply(all, Filter{Protocol: TCP})
	if len(tcp) != 4 {
		t.Fatalf("TCP filter matched %d ports, want 4", len(tcp))
	}
	for _, p := range tcp {
		if p.Protocol != TCP {
			t.Errorf("TCP filter returned a %v port", p.Protocol)
		}
	}

	udp := Apply(all, Filter{Protocol: UDP})
	if len(udp) != 2 {
		t.Fatalf("UDP filter matched %d ports, want 2", len(udp))
	}
	for _, p := range udp {
		if p.Protocol != UDP {
			t.Errorf("UDP filter returned a %v port", p.Protocol)
		}
	}
}

func TestFilterByPort(t *testing.T) {
	tests := []struct {
		port uint16
		want []uint16
	}{
		{port: 3000, want: []uint16{3000}},
		{port: 53, want: []uint16{53}},
		{port: 5432, want: []uint16{5432}},
		{port: 9999, want: []uint16{}},
	}
	for _, tt := range tests {
		got := Apply(fixture(), Filter{LocalPort: tt.port})
		if len(got) != len(tt.want) {
			t.Errorf("port %d matched %v, want %v", tt.port, portsByLocalPort(got), tt.want)
			continue
		}
		for i := range got {
			if got[i].LocalPort != tt.want[i] {
				t.Errorf("port %d matched %v, want %v", tt.port, portsByLocalPort(got), tt.want)
				break
			}
		}
	}
}

func TestFilterByPID(t *testing.T) {
	got := Apply(fixture(), Filter{PID: 9144})
	if len(got) != 1 || got[0].Process != "postgres.exe" {
		t.Errorf("PID filter matched %+v, want the postgres socket", got)
	}

	if got := Apply(fixture(), Filter{PID: 999999}); len(got) != 0 {
		t.Errorf("unknown PID matched %d ports, want 0", len(got))
	}
}

func TestFilterByProcessIsCaseInsensitive(t *testing.T) {
	for _, needle := range []string{"node", "NODE", "Node", "node.exe", "ODE.EXE"} {
		got := Apply(fixture(), Filter{ProcessSub: needle})
		if len(got) != 1 || got[0].Process != "node.exe" {
			t.Errorf("process filter %q matched %+v, want the node socket", needle, got)
		}
	}

	// A partial match must still work, and a miss must match nothing.
	if got := Apply(fixture(), Filter{ProcessSub: "svc"}); len(got) != 1 {
		t.Errorf("partial process filter matched %d ports, want 1", len(got))
	}
	if got := Apply(fixture(), Filter{ProcessSub: "nothing-here"}); len(got) != 0 {
		t.Errorf("bogus process filter matched %d ports, want 0", len(got))
	}
}

func TestFilterCombinesConditions(t *testing.T) {
	// Only the TCP socket of node.exe should survive.
	got := Apply(fixture(), Filter{Protocol: TCP, ProcessSub: "node"})
	if len(got) != 1 || got[0].LocalPort != 3000 {
		t.Errorf("combined filter matched %+v, want only 127.0.0.1:3000", got)
	}

	// Contradictory conditions must match nothing.
	if got := Apply(fixture(), Filter{Protocol: UDP, PID: 9144}); len(got) != 0 {
		t.Errorf("contradictory filter matched %d ports, want 0", len(got))
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	all := fixture()
	before := len(all)

	_ = Apply(all, Filter{Protocol: UDP})

	if len(all) != before {
		t.Errorf("Apply changed the input length: %d, want %d", len(all), before)
	}
}

func TestCountUnknownProcesses(t *testing.T) {
	all := fixture()
	if got := CountUnknownProcesses(all); got != 1 {
		t.Errorf("CountUnknownProcesses = %d, want 1", got)
	}

	known := []Port{{Process: "a"}, {Process: "b"}}
	if got := CountUnknownProcesses(known); got != 0 {
		t.Errorf("CountUnknownProcesses = %d, want 0", got)
	}

	if got := CountUnknownProcesses(nil); got != 0 {
		t.Errorf("CountUnknownProcesses(nil) = %d, want 0", got)
	}
}
