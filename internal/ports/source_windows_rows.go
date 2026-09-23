//go:build windows

package ports

import (
	"encoding/binary"
	"net"
	"unsafe"
)

// convert turns one raw row into a domain record. ok is false when the row
// cannot be represented.
func (k rowKind) convert(row unsafe.Pointer) (Record, bool) {
	switch k {
	case rowKindTCP4:
		return convertTCP4(row)
	case rowKindTCP6:
		return convertTCP6(row)
	case rowKindUDP4:
		return convertUDP4(row)
	default:
		return convertUDP6(row)
	}
}

// convertTCP4 decodes a MIB_TCPROW_OWNER_PID.
//
// The packed row is six little-endian uint32 values: state, local address,
// local port, remote address, remote port and the owning PID. Both the address
// and the port are stored as a whole word, with the port in the low 16 bits.
func convertTCP4(row unsafe.Pointer) (Record, bool) {
	u := (*[6]uint32)(row)
	state, ok := tcpState(u[0])
	if !ok {
		return Record{}, false
	}
	return Record{
		Protocol:   TCP,
		LocalIP:    ipv4String(u[1]),
		LocalPort:  uint16(u[2] & 0xffff),
		RemoteIP:   ipv4String(u[3]),
		RemotePort: uint16(u[4] & 0xffff),
		State:      state,
		PID:        u[5],
	}, true
}

// convertTCP6 decodes a MIB_TCP6ROW_OWNER_PID.
//
// The row is 56 bytes: a 16-byte local address, a 4-byte local scope id, a
// 4-byte local port word, the same 24-byte group for the remote endpoint, then
// the state and the owning PID. Ports are 32-bit words with the port in the low
// 16 bits, and the addresses are not naturally aligned so they are read
// byte-wise.
func convertTCP6(row unsafe.Pointer) (Record, bool) {
	b := unsafe.Slice((*byte)(row), tcpRowSizeV6)

	state, ok := tcpState(binary.LittleEndian.Uint32(b[48:52]))
	if !ok {
		return Record{}, false
	}

	return Record{
		Protocol:   TCP,
		LocalIP:    ipv6String(b[0:16]),
		LocalPort:  uint16(binary.LittleEndian.Uint32(b[20:24]) & 0xffff),
		RemoteIP:   ipv6String(b[24:40]),
		RemotePort: uint16(binary.LittleEndian.Uint32(b[44:48]) & 0xffff),
		State:      state,
		PID:        binary.LittleEndian.Uint32(b[52:56]),
	}, true
}

// convertUDP4 decodes a MIB_UDPROW_OWNER_PID: local address, local port and
// owning PID.
//
// UDP has no connection state machine, so the state stays StateNone. Reporting
// LISTENING would invent information the kernel does not provide.
func convertUDP4(row unsafe.Pointer) (Record, bool) {
	u := (*[3]uint32)(row)
	return Record{
		Protocol:  UDP,
		LocalIP:   ipv4String(u[0]),
		LocalPort: uint16(u[1] & 0xffff),
		State:     StateNone,
		PID:       u[2],
	}, true
}

// convertUDP6 decodes a MIB_UDP6ROW_OWNER_PID: a 16-byte address, a 4-byte
// scope id, a 4-byte port word and the owning PID.
func convertUDP6(row unsafe.Pointer) (Record, bool) {
	b := unsafe.Slice((*byte)(row), udpRowSizeV6)

	return Record{
		Protocol:  UDP,
		LocalIP:   ipv6String(b[0:16]),
		LocalPort: uint16(binary.LittleEndian.Uint32(b[20:24]) & 0xffff),
		State:     StateNone,
		PID:       binary.LittleEndian.Uint32(b[24:28]),
	}, true
}

// ipv4String renders an IPv4 address stored as a whole little-endian word.
func ipv4String(word uint32) string {
	ip := net.IPv4(
		byte(word&0xff),
		byte(word>>8&0xff),
		byte(word>>16&0xff),
		byte(word>>24&0xff),
	)
	return ip.String()
}

// ipv6String renders 16 raw address bytes.
func ipv6String(raw []byte) string {
	var addr [16]byte
	copy(addr[:], raw)
	return net.IP(addr[:]).String()
}

// tcpState maps a MIB_TCP_STATE value onto the domain model. ok is false for
// values that are reserved or that nselecttrace does not recognise.
func tcpState(v uint32) (State, bool) {
	switch v {
	case 1: // MIB_TCP_STATE_CLOSED
		return StateClosed, true
	case 2: // MIB_TCP_STATE_LISTEN
		return StateListen, true
	case 3: // MIB_TCP_STATE_SYN_SENT
		return StateSynSent, true
	case 4: // MIB_TCP_STATE_SYN_RCVD
		return StateSynReceived, true
	case 5: // MIB_TCP_STATE_ESTAB
		return StateEstablished, true
	case 6: // MIB_TCP_STATE_FIN_WAIT1
		return StateFinWait1, true
	case 7: // MIB_TCP_STATE_FIN_WAIT2
		return StateFinWait2, true
	case 8: // MIB_TCP_STATE_CLOSE_WAIT
		return StateCloseWait, true
	case 9: // MIB_TCP_STATE_CLOSING
		return StateClosing, true
	case 10: // MIB_TCP_STATE_LAST_ACK
		return StateLastAck, true
	case 11: // MIB_TCP_STATE_TIME_WAIT
		return StateTimeWait, true
	case 12: // MIB_TCP_STATE_DELETE_TCB
		return StateDeleteTCB, true
	default:
		// 0 is MIB_TCP_STATE_CLOSED on some SDK revisions but is not documented
		// as such. Anything else is a state nselecttrace does not know, and guessing
		// would be worse than admitting it.
		return StateUnknown, false
	}
}
