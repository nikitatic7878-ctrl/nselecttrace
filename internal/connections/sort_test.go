package connections

import (
	"testing"

	"nselecttrace/internal/ports"
)

func TestSortIsDeterministic(t *testing.T) {
	all := fixture()

	first := Sort(OnlyWithRemote(all))
	second := Sort(OnlyWithRemote(all))

	if len(first) != len(second) {
		t.Fatalf("lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("Sort is not deterministic at %d:\n%+v\n%+v", i, first[i], second[i])
		}
	}
}

func TestSortOrdersByProtocolThenEndpoints(t *testing.T) {
	all := Select(fixture())

	// Expected order: protocol, local IP, local port, remote IP, remote port,
	// state, PID. The IPv6 socket sorts last within the TCP group because
	// "fe80::1234" follows "192.168.1.6" textually.
	want := []uint16{52143, 52341, 52342, 53122, 53123, 53124, 53126}
	got := connectionPorts(all)

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order = %v, want %v", got, want)
			break
		}
	}
}

func TestSortResultIsOrdered(t *testing.T) {
	all := Select(fixture())
	for i := 1; i < len(all); i++ {
		if less(all[i], all[i-1]) {
			t.Errorf("result is not sorted at %d: %+v then %+v", i, all[i-1], all[i])
		}
	}
}

func TestSortDoesNotMutateInput(t *testing.T) {
	all := fixture()
	before := make([]ports.Port, len(all))
	copy(before, all)

	_ = Sort(all)

	for i := range all {
		if all[i] != before[i] {
			t.Fatalf("Sort mutated its input at %d", i)
		}
	}
}

func TestCountUnknownProcesses(t *testing.T) {
	all := Select(fixture())

	// Exactly one connection in the fixture has no owner: the TIME_WAIT socket.
	if got := CountUnknownProcesses(all); got != 1 {
		t.Errorf("CountUnknownProcesses = %d, want 1", got)
	}
}
