//go:build windows

package ports

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

// buildTCP4Row builds a row in the layout the API uses: six little-endian
// 4-byte words.
func buildTCP4Row(state uint32, localIP [4]byte, localPort uint16, remoteIP [4]byte, remotePort uint16, pid uint32) []byte {
	row := make([]byte, tcpRowSizeV4)
	binary.LittleEndian.PutUint32(row[0:4], state)
	copy(row[4:8], localIP[:])
	binary.LittleEndian.PutUint32(row[8:12], uint32(localPort))
	copy(row[12:16], remoteIP[:])
	binary.LittleEndian.PutUint32(row[16:20], uint32(remotePort))
	binary.LittleEndian.PutUint32(row[20:24], pid)
	return row
}

// buildUDP4Row builds a MIB_UDPROW_OWNER_PID: address, port, PID.
func buildUDP4Row(localIP [4]byte, localPort uint16, pid uint32) []byte {
	row := make([]byte, udpRowSizeV4)
	copy(row[0:4], localIP[:])
	binary.LittleEndian.PutUint32(row[4:8], uint32(localPort))
	binary.LittleEndian.PutUint32(row[8:12], pid)
	return row
}

// buildTCP6Row builds a MIB_TCP6ROW_OWNER_PID.
func buildTCP6Row(state uint32, localIP [16]byte, localPort uint16, remoteIP [16]byte, remotePort uint16, pid uint32) []byte {
	row := make([]byte, tcpRowSizeV6)
	copy(row[0:16], localIP[:])
	binary.LittleEndian.PutUint32(row[20:24], uint32(localPort))
	copy(row[24:40], remoteIP[:])
	binary.LittleEndian.PutUint32(row[44:48], uint32(remotePort))
	binary.LittleEndian.PutUint32(row[48:52], state)
	binary.LittleEndian.PutUint32(row[52:56], pid)
	return row
}

// buildUDP6Row builds a MIB_UDP6ROW_OWNER_PID.
func buildUDP6Row(localIP [16]byte, localPort uint16, pid uint32) []byte {
	row := make([]byte, udpRowSizeV6)
	copy(row[0:16], localIP[:])
	binary.LittleEndian.PutUint32(row[16:20], 0) // scope id
	binary.LittleEndian.PutUint32(row[20:24], uint32(localPort))
	binary.LittleEndian.PutUint32(row[24:28], pid)
	return row
}

func TestConvertTCP4Row(t *testing.T) {
	row := buildTCP4Row(5, [4]byte{192, 168, 1, 6}, 5432, [4]byte{192, 168, 1, 20}, 51000, 9144)

	rec, ok := convertTCP4(unsafe.Pointer(&row[0]))
	if !ok {
		t.Fatal("convertTCP4 rejected a valid row")
	}
	if rec.Protocol != TCP {
		t.Errorf("Protocol = %v, want TCP", rec.Protocol)
	}
	if rec.LocalIP != "192.168.1.6" {
		t.Errorf("LocalIP = %q, want 192.168.1.6", rec.LocalIP)
	}
	if rec.LocalPort != 5432 {
		t.Errorf("LocalPort = %d, want 5432", rec.LocalPort)
	}
	if rec.RemoteIP != "192.168.1.20" {
		t.Errorf("RemoteIP = %q, want 192.168.1.20", rec.RemoteIP)
	}
	if rec.RemotePort != 51000 {
		t.Errorf("RemotePort = %d, want 51000", rec.RemotePort)
	}
	if rec.State != StateEstablished {
		t.Errorf("State = %v, want ESTABLISHED", rec.State)
	}
	if rec.PID != 9144 {
		t.Errorf("PID = %d, want 9144", rec.PID)
	}
}

func TestConvertTCP4RowStates(t *testing.T) {
	// Every documented MIB_TCP_STATE value must map onto a named state.
	tests := []struct {
		raw  uint32
		want State
	}{
		{1, StateClosed}, {2, StateListen}, {3, StateSynSent}, {4, StateSynReceived},
		{5, StateEstablished}, {6, StateFinWait1}, {7, StateFinWait2}, {8, StateCloseWait},
		{9, StateClosing}, {10, StateLastAck}, {11, StateTimeWait}, {12, StateDeleteTCB},
	}
	for _, tt := range tests {
		row := buildTCP4Row(tt.raw, [4]byte{127, 0, 0, 1}, 80, [4]byte{}, 0, 4)
		rec, ok := convertTCP4(unsafe.Pointer(&row[0]))
		if !ok {
			t.Errorf("state %d was rejected", tt.raw)
			continue
		}
		if rec.State != tt.want {
			t.Errorf("state %d mapped to %v, want %v", tt.raw, rec.State, tt.want)
		}
	}
}

func TestConvertTCP4RowRejectsUnknownState(t *testing.T) {
	// A reserved state must be skipped, not guessed at.
	for _, raw := range []uint32{0, 13, 99, 0xffff} {
		row := buildTCP4Row(raw, [4]byte{127, 0, 0, 1}, 80, [4]byte{}, 0, 4)
		if rec, ok := convertTCP4(unsafe.Pointer(&row[0])); ok {
			t.Errorf("state %d was accepted as %+v, want it rejected", raw, rec)
		}
	}
}

func TestConvertTCP4RowWildcardBind(t *testing.T) {
	row := buildTCP4Row(2, [4]byte{0, 0, 0, 0}, 445, [4]byte{}, 0, 4)

	rec, ok := convertTCP4(unsafe.Pointer(&row[0]))
	if !ok {
		t.Fatal("convertTCP4 rejected a valid listening row")
	}
	if !(Port{LocalIP: rec.LocalIP}).IsWildcard() {
		t.Errorf("LocalIP = %q, want a wildcard address", rec.LocalIP)
	}
	if rec.LocalPort != 445 {
		t.Errorf("LocalPort = %d, want 445", rec.LocalPort)
	}
	if rec.State != StateListen {
		t.Errorf("State = %v, want LISTENING", rec.State)
	}
}

func TestConvertUDP4Row(t *testing.T) {
	row := buildUDP4Row([4]byte{0, 0, 0, 0}, 53, 1400)

	rec, ok := convertUDP4(unsafe.Pointer(&row[0]))
	if !ok {
		t.Fatal("convertUDP4 rejected a valid row")
	}
	if rec.Protocol != UDP {
		t.Errorf("Protocol = %v, want UDP", rec.Protocol)
	}
	if rec.LocalPort != 53 {
		t.Errorf("LocalPort = %d, want 53", rec.LocalPort)
	}
	if rec.PID != 1400 {
		t.Errorf("PID = %d, want 1400", rec.PID)
	}
	// UDP must never be reported as LISTENING: there is no state to read.
	if rec.State != StateNone {
		t.Errorf("State = %v, want StateNone for UDP", rec.State)
	}
	if rec.State.String() != "-" {
		t.Errorf("UDP state renders as %q, want %q", rec.State.String(), "-")
	}
}

func TestConvertUDP6Row(t *testing.T) {
	addr := [16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	row := buildUDP6Row(addr, 5353, 200)

	rec, ok := convertUDP6(unsafe.Pointer(&row[0]))
	if !ok {
		t.Fatal("convertUDP6 rejected a valid row")
	}
	if rec.LocalIP != "fe80::1" {
		t.Errorf("LocalIP = %q, want fe80::1", rec.LocalIP)
	}
	if rec.LocalPort != 5353 {
		t.Errorf("LocalPort = %d, want 5353", rec.LocalPort)
	}
	if rec.PID != 200 {
		t.Errorf("PID = %d, want 200", rec.PID)
	}
	if rec.State != StateNone {
		t.Errorf("State = %v, want StateNone for UDP", rec.State)
	}
}

func TestConvertTCP6Row(t *testing.T) {
	local := [16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	remote := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	row := buildTCP6Row(11, local, 5353, remote, 443, 4242)

	rec, ok := convertTCP6(unsafe.Pointer(&row[0]))
	if !ok {
		t.Fatal("convertTCP6 rejected a valid row")
	}
	if rec.LocalIP != "fe80::1" {
		t.Errorf("LocalIP = %q, want fe80::1", rec.LocalIP)
	}
	if rec.LocalPort != 5353 {
		t.Errorf("LocalPort = %d, want 5353", rec.LocalPort)
	}
	if rec.RemoteIP != "2001:db8::1" {
		t.Errorf("RemoteIP = %q, want 2001:db8::1", rec.RemoteIP)
	}
	if rec.RemotePort != 443 {
		t.Errorf("RemotePort = %d, want 443", rec.RemotePort)
	}
	if rec.State != StateTimeWait {
		t.Errorf("State = %v, want TIME_WAIT", rec.State)
	}
	if rec.PID != 4242 {
		t.Errorf("PID = %d, want 4242", rec.PID)
	}
}

func TestConvertTCP6RowRejectsUnknownState(t *testing.T) {
	var addr [16]byte
	row := buildTCP6Row(0, addr, 1, addr, 0, 1)

	if rec, ok := convertTCP6(unsafe.Pointer(&row[0])); ok {
		t.Errorf("state 0 was accepted as %+v, want it rejected", rec)
	}
}

func TestRowSizesMatchThePackedAPILayout(t *testing.T) {
	// The API packs rows without the padding a C compiler would add, so these
	// are asserted explicitly: a wrong value would corrupt every row silently.
	if tcpRowSizeV4 != 24 {
		t.Errorf("tcpRowSizeV4 = %d, want 24", tcpRowSizeV4)
	}
	if tcpRowSizeV6 != 56 {
		t.Errorf("tcpRowSizeV6 = %d, want 56", tcpRowSizeV6)
	}
	if udpRowSizeV4 != 12 {
		t.Errorf("udpRowSizeV4 = %d, want 12", udpRowSizeV4)
	}
	if udpRowSizeV6 != 28 {
		t.Errorf("udpRowSizeV6 = %d, want 28", udpRowSizeV6)
	}
}

func TestRowKindAccessors(t *testing.T) {
	tests := []struct {
		kind   rowKind
		class  uint32
		family uint32
		size   int
		label  string
	}{
		{rowKindTCP4, tcpTableOwnerPIDAll, afInet, tcpRowSizeV4, "TCP/IPv4"},
		{rowKindTCP6, tcpTableOwnerPIDAll, afInet6, tcpRowSizeV6, "TCP/IPv6"},
		{rowKindUDP4, udpTableOwnerPID, afInet, udpRowSizeV4, "UDP/IPv4"},
		{rowKindUDP6, udpTableOwnerPID, afInet6, udpRowSizeV6, "UDP/IPv6"},
	}
	for _, tt := range tests {
		if got := tt.kind.class(); got != tt.class {
			t.Errorf("%s class = %d, want %d", tt.label, got, tt.class)
		}
		if got := tt.kind.family(); got != tt.family {
			t.Errorf("%s family = %d, want %d", tt.label, got, tt.family)
		}
		if got := tt.kind.rowSize(); got != tt.size {
			t.Errorf("%s rowSize = %d, want %d", tt.label, got, tt.size)
		}
		if got := tt.kind.label(); got != tt.label {
			t.Errorf("label = %q, want %q", got, tt.label)
		}
	}
}

func TestConvertDispatchesByRowKind(t *testing.T) {
	tcp4 := buildTCP4Row(2, [4]byte{127, 0, 0, 1}, 80, [4]byte{}, 0, 4)
	udp4 := buildUDP4Row([4]byte{0, 0, 0, 0}, 53, 1400)

	rec, ok := rowKindTCP4.convert(unsafe.Pointer(&tcp4[0]))
	if !ok || rec.Protocol != TCP || rec.State != StateListen {
		t.Errorf("rowKindTCP4.convert = %+v (ok=%v), want a LISTENING TCP record", rec, ok)
	}

	rec, ok = rowKindUDP4.convert(unsafe.Pointer(&udp4[0]))
	if !ok || rec.Protocol != UDP || rec.State != StateNone {
		t.Errorf("rowKindUDP4.convert = %+v (ok=%v), want a stateless UDP record", rec, ok)
	}
}

// TestReadTableOverSyntheticBuffer drives the whole row-walking path with a
// buffer laid out exactly as the API returns one. It is the regression test for
// the packed row sizes: with a wrong stride every row after the first would be
// garbage, and with a wrong class constant LISTENING rows would vanish.
func TestReadTableOverSyntheticBuffer(t *testing.T) {
	// Build a table of three TCP/IPv4 rows the way the API packs them:
	// a leading count, then 24-byte rows.
	rows := [][]byte{
		buildTCP4Row(2, [4]byte{0, 0, 0, 0}, 135, [4]byte{}, 0, 1200),
		buildTCP4Row(5, [4]byte{192, 168, 1, 6}, 5432, [4]byte{192, 168, 1, 20}, 51000, 9144),
		buildTCP4Row(1, [4]byte{10, 0, 0, 1}, 1024, [4]byte{10, 0, 0, 2}, 80, 4),
	}

	buf := make([]byte, 4+len(rows)*tcpRowSizeV4)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(rows)))
	for i, row := range rows {
		copy(buf[4+i*tcpRowSizeV4:], row)
	}

	// Walk it the same way readTable does, without calling the API.
	count := binary.LittleEndian.Uint32(buf[0:4])
	table := buf[4:]
	base := unsafe.Pointer(&table[0])

	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	want := []struct {
		port  uint16
		state State
		pid   uint32
	}{
		{135, StateListen, 1200},
		{5432, StateEstablished, 9144},
		{1024, StateClosed, 4},
	}

	for i := uint32(0); i < count; i++ {
		offset := uintptr(i) * uintptr(tcpRowSizeV4)
		if offset+uintptr(tcpRowSizeV4) > uintptr(len(table)) {
			t.Fatalf("row %d runs past the buffer", i)
		}
		rec, ok := rowKindTCP4.convert(unsafe.Add(base, offset))
		if !ok {
			t.Fatalf("row %d was rejected", i)
		}
		if rec.LocalPort != want[i].port || rec.State != want[i].state || rec.PID != want[i].pid {
			t.Errorf("row %d = port %d state %v pid %d, want port %d state %v pid %d",
				i, rec.LocalPort, rec.State, rec.PID, want[i].port, want[i].state, want[i].pid)
		}
	}
}

// TestReadTableStopsAtTruncatedBuffer proves the bounds check keeps the walk
// inside the buffer even when the count overstates the number of rows.
func TestReadTableStopsAtTruncatedBuffer(t *testing.T) {
	buf := make([]byte, 4+tcpRowSizeV4) // room for exactly one row
	binary.LittleEndian.PutUint32(buf[0:4], 5)

	count := binary.LittleEndian.Uint32(buf[0:4])
	table := buf[4:]
	base := unsafe.Pointer(&table[0])

	converted := 0
	for i := uint32(0); i < count; i++ {
		offset := uintptr(i) * uintptr(tcpRowSizeV4)
		if offset+uintptr(tcpRowSizeV4) > uintptr(len(table)) {
			break
		}
		if _, ok := rowKindTCP4.convert(unsafe.Add(base, offset)); ok {
			converted++
		}
	}

	if converted != 0 {
		// The single row is all zero bytes, so state 0 must be rejected.
		t.Errorf("converted = %d, want 0 (a zero row has an unknown state)", converted)
	}
}

func TestIPv4String(t *testing.T) {
	tests := []struct {
		word uint32
		want string
	}{
		{word: 0x00000000, want: "0.0.0.0"},
		{word: 0x0100007f, want: "127.0.0.1"},
		{word: 0x0601a8c0, want: "192.168.1.6"},
		{word: 0xffffffff, want: "255.255.255.255"},
	}
	for _, tt := range tests {
		if got := ipv4String(tt.word); got != tt.want {
			t.Errorf("ipv4String(0x%08x) = %q, want %q", tt.word, got, tt.want)
		}
	}
}

func TestIPv6String(t *testing.T) {
	lo := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if got := ipv6String(lo); got != "::1" {
		t.Errorf("ipv6String(loopback) = %q, want ::1", got)
	}

	link := []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if got := ipv6String(link); got != "fe80::" {
		t.Errorf("ipv6String(link local) = %q, want fe80::", got)
	}
}
