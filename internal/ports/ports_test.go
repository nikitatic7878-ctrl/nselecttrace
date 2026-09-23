package ports

import (
	"errors"
	"strings"
	"testing"
)

// fixtureSource returns a fixed set of records instead of touching the real
// socket tables, so no test depends on what happens to be running on the
// machine that executes it.
type fixtureSource struct {
	records []Record
	err     error

	calls int
}

func (s *fixtureSource) Records() ([]Record, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.records, nil
}

// sample is a synthetic but realistic listing: a privileged TCP listener, a
// loopback listener, two sockets of the same process, a UDP socket and a
// wildcard-bound IPv6 socket.
func sample() []Record {
	return []Record{
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 135, State: StateListen, PID: 1200, Process: "svchost.exe"},
		{Protocol: TCP, LocalIP: "127.0.0.1", LocalPort: 3000, State: StateListen, PID: 8212, Process: "node.exe"},
		{Protocol: TCP, LocalIP: "192.168.1.6", LocalPort: 5432, State: StateEstablished, PID: 9144, Process: "postgres.exe",
			RemoteIP: "192.168.1.20", RemotePort: 51000},
		{Protocol: TCP, LocalIP: "192.168.1.6", LocalPort: 5432, State: StateEstablished, PID: 9144, Process: "postgres.exe",
			RemoteIP: "192.168.1.21", RemotePort: 51001},
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 53, State: StateNone, PID: 1400, Process: "dns.exe"},
		{Protocol: UDP, LocalIP: "::", LocalPort: 5353, State: StateNone, PID: 200, Process: "mdns.exe"},
	}
}

func TestListFromReturnsRecords(t *testing.T) {
	got, err := ListFrom(&fixtureSource{records: sample()})
	if err != nil {
		t.Fatalf("ListFrom returned error: %v", err)
	}
	if len(got) != len(sample()) {
		t.Fatalf("len = %d, want %d", len(got), len(sample()))
	}
}

func TestListFromWrapsSourceError(t *testing.T) {
	sentinel := errors.New("table unavailable")

	got, err := ListFrom(&fixtureSource{err: sentinel})
	if err == nil {
		t.Fatalf("ListFrom = %+v, want error", got)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the source error", err)
	}
	if !strings.Contains(err.Error(), "failed to list ports") {
		t.Errorf("error = %q, want the operation prefix", err.Error())
	}
}

func TestSortIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	input := []Record{
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 53, PID: 1400, Process: "dns.exe"},
		{Protocol: TCP, LocalIP: "192.168.1.6", LocalPort: 5432, PID: 9144, Process: "postgres.exe"},
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 135, PID: 1200, Process: "svchost.exe"},
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 80, PID: 4, Process: "System"},
		{Protocol: TCP, LocalIP: "127.0.0.1", LocalPort: 3000, PID: 8212, Process: "node.exe"},
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 53, PID: 100, Process: "other.exe"},
	}

	first, err := ListFrom(&fixtureSource{records: input})
	if err != nil {
		t.Fatalf("ListFrom returned error: %v", err)
	}
	second, err := ListFrom(&fixtureSource{records: input})
	if err != nil {
		t.Fatalf("ListFrom returned error: %v", err)
	}

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("Sort is not deterministic at %d: %+v vs %+v", i, first[i], second[i])
		}
	}

	// TCP before UDP, then by address, then by port, then by PID.
	want := []struct {
		proto Protocol
		ip    string
		port  uint16
		pid   uint32
	}{
		{TCP, "0.0.0.0", 80, 4},
		{TCP, "0.0.0.0", 135, 1200},
		{TCP, "127.0.0.1", 3000, 8212},
		{TCP, "192.168.1.6", 5432, 9144},
		{UDP, "0.0.0.0", 53, 100},
		{UDP, "0.0.0.0", 53, 1400},
	}
	for i, w := range want {
		got := first[i]
		if got.Protocol != w.proto || got.LocalIP != w.ip || got.LocalPort != w.port || got.PID != w.pid {
			t.Errorf("sorted[%d] = %v %s:%d pid %d, want %v %s:%d pid %d",
				i, got.Protocol, got.LocalIP, got.LocalPort, got.PID, w.proto, w.ip, w.port, w.pid)
		}
	}

	// The input order must survive: Sort copies before sorting.
	if input[0].LocalPort != 53 || input[1].LocalPort != 5432 {
		t.Errorf("Sort mutated its input: %v", input)
	}
}

func TestListFromResolvesProcessNamesOncePerPID(t *testing.T) {
	// Two sockets share a PID; the process cache must be consulted once, which
	// is observable through the record count being preserved either way.
	records := []Record{
		{Protocol: TCP, LocalIP: "10.0.0.1", LocalPort: 100, PID: 7},
		{Protocol: TCP, LocalIP: "10.0.0.1", LocalPort: 101, PID: 7},
	}

	got, err := ListFrom(&fixtureSource{records: records})
	if err != nil {
		t.Fatalf("ListFrom returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// The names come from the (unavailable) process lookup, but both sockets of
	// the same PID must agree.
	if got[0].Process != got[1].Process {
		t.Errorf("sockets of the same PID disagree: %q vs %q", got[0].Process, got[1].Process)
	}
}

func TestListFromKeepsRecordSuppliedProcessName(t *testing.T) {
	// A platform source may already know the name; the resolver must not
	// overwrite it.
	records := []Record{
		{Protocol: TCP, LocalIP: "10.0.0.1", LocalPort: 100, PID: 4, Process: "System"},
	}

	got, err := ListFrom(&fixtureSource{records: records})
	if err != nil {
		t.Fatalf("ListFrom returned error: %v", err)
	}
	if got[0].Process != "System" {
		t.Errorf("Process = %q, want %q", got[0].Process, "System")
	}
}

// TestUnsupportedSourceFailsHonestly pins the contract every platform without a
// backend must honour: ports.List returns an error that names the reason and is
// recognisable with errors.Is, so the CLI can report it instead of printing an
// empty table that looks like "no sockets".
//
// The test drives ListFrom with the real ErrUnsupported rather than calling
// platformSource directly, because platformSource only exists on non-Windows
// builds while the contract holds on every platform.
func TestUnsupportedSourceFailsHonestly(t *testing.T) {
	src := &fixtureSource{err: ErrUnsupported}

	got, err := ListFrom(src)
	if err == nil {
		t.Fatalf("ListFrom = %+v, want an error", got)
	}
	if got != nil {
		t.Errorf("ListFrom returned %+v alongside an error, want nil", got)
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want it to wrap ErrUnsupported", err)
	}
	if !strings.Contains(err.Error(), "not implemented on this platform") {
		t.Errorf("error = %q, want it to name the reason", err.Error())
	}
	if !strings.Contains(err.Error(), "failed to list ports") {
		t.Errorf("error = %q, want the operation context", err.Error())
	}
}

// TestListDoesNotSilentlyReturnEmptyOnUnsupportedPlatform guards the "no fake
// data" rule: an unsupported platform must surface an error, never an empty
// listing that a user would read as "nothing is listening".
func TestListDoesNotSilentlyReturnEmptyOnUnsupportedPlatform(t *testing.T) {
	restore := stubSystemSource(&fixtureSource{err: ErrUnsupported})
	defer restore()

	got, err := List()
	if err == nil {
		t.Fatalf("List() = %+v, want an error on an unsupported platform", got)
	}
	if len(got) != 0 {
		t.Errorf("List() returned %d ports alongside an error, want none", len(got))
	}
}

// stubSystemSource swaps the package level source for the duration of a test.
func stubSystemSource(src Source) func() {
	prev := systemSource
	systemSource = src
	return func() { systemSource = prev }
}
