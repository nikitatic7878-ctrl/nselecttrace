package ping

// ICMP message types and header layout shared by both address families.
//
// The ICMPv4 echo header is type(1) code(1) checksum(2) identifier(2)
// sequence(2). ICMPv6 uses the same shape with type 128/129 and a checksum that
// the kernel computes for a raw IPv6 socket.
const (
	icmpHeaderSize = 8

	icmpv4EchoRequest = 8
	icmpv4EchoReply   = 0

	icmpv6EchoRequest = 128
	icmpv6EchoReply   = 129
)

// protoReply maps an echo request type to its reply type.
func protoReply(request uint8) uint8 {
	if request == icmpv6EchoRequest {
		return icmpv6EchoReply
	}
	return icmpv4EchoReply
}

// buildEchoRequest builds an ICMP echo request with a valid checksum.
//
// The checksum is computed for ICMPv4 only. For ICMPv6 the kernel calculates it
// for a raw socket, and a hand-computed value would be overwritten; computing it
// anyway would be harmless but misleading, so it is left as zero.
func buildEchoRequest(requestType uint8, identifier uint16, sequence int) []byte {
	msg := make([]byte, icmpHeaderSize+payloadSize)

	msg[0] = requestType
	msg[1] = 0 // code is always 0 for echo
	msg[4] = byte(identifier >> 8)
	msg[5] = byte(identifier)
	msg[6] = byte(uint16(sequence) >> 8)
	msg[7] = byte(sequence)

	// A predictable payload, so a reply can be recognised as an echo of this
	// request rather than of anything else on the host.
	for i := icmpHeaderSize; i < len(msg); i++ {
		msg[i] = byte(i - icmpHeaderSize)
	}

	if requestType == icmpv4EchoRequest {
		sum := checksum(msg)
		msg[2] = byte(sum >> 8)
		msg[3] = byte(sum)
	}
	return msg
}

// checksum computes the one's complement 16-bit checksum used by ICMPv4.
func checksum(msg []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(msg); i += 2 {
		sum += uint32(msg[i])<<8 | uint32(msg[i+1])
	}
	// A trailing odd byte is padded with a zero low byte.
	if len(msg)%2 == 1 {
		sum += uint32(msg[len(msg)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// replySequence extracts the sequence number from a received ICMP message.
func replySequence(msg []byte) uint16 {
	if len(msg) < icmpHeaderSize {
		return 0
	}
	return uint16(msg[6])<<8 | uint16(msg[7])
}
