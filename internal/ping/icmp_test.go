package ping

import (
	"net"
	"testing"
)

func TestBuildEchoRequestHeader(t *testing.T) {
	msg := buildEchoRequest(icmpv4EchoRequest, 0x1234, 7)

	if len(msg) != icmpHeaderSize+payloadSize {
		t.Fatalf("len = %d, want %d (a conventional 64-byte request)", len(msg), icmpHeaderSize+payloadSize)
	}
	if msg[0] != icmpv4EchoRequest {
		t.Errorf("type = %d, want %d", msg[0], icmpv4EchoRequest)
	}
	if msg[1] != 0 {
		t.Errorf("code = %d, want 0", msg[1])
	}

	identifier := uint16(msg[4])<<8 | uint16(msg[5])
	if identifier != 0x1234 {
		t.Errorf("identifier = 0x%04x, want 0x1234", identifier)
	}
	if got := replySequence(msg); got != 7 {
		t.Errorf("sequence = %d, want 7", got)
	}
}

func TestBuildEchoRequestChecksumIsValid(t *testing.T) {
	// A correct checksum makes the sum over the whole message zero.
	msg := buildEchoRequest(icmpv4EchoRequest, 1, 1)

	var sum uint32
	for i := 0; i+1 < len(msg); i += 2 {
		sum += uint32(msg[i])<<8 | uint32(msg[i+1])
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	if uint16(sum) != 0xffff {
		t.Errorf("checksum over the message = 0x%04x, want 0xffff", uint16(sum))
	}
}

func TestBuildEchoRequestPayloadIsPredictable(t *testing.T) {
	// The payload must be reproducible so a reply can be recognised as an echo
	// of this request.
	first := buildEchoRequest(icmpv4EchoRequest, 1, 1)
	second := buildEchoRequest(icmpv4EchoRequest, 1, 1)

	if string(first) != string(second) {
		t.Error("the same request produced different bytes")
	}
	for i := icmpHeaderSize; i < len(first); i++ {
		if first[i] != byte(i-icmpHeaderSize) {
			t.Fatalf("payload byte %d = %d, want %d", i, first[i], byte(i-icmpHeaderSize))
		}
	}
}

func TestBuildEchoRequestV6LeavesChecksumToTheKernel(t *testing.T) {
	// For ICMPv6 on a raw socket the kernel computes the checksum, so the field
	// is left zero rather than filled with a value that would be overwritten.
	msg := buildEchoRequest(icmpv6EchoRequest, 1, 1)

	if msg[0] != icmpv6EchoRequest {
		t.Errorf("type = %d, want %d", msg[0], icmpv6EchoRequest)
	}
	if msg[2] != 0 || msg[3] != 0 {
		t.Errorf("checksum = 0x%02x%02x, want 0", msg[2], msg[3])
	}
}

func TestChecksum(t *testing.T) {
	// A message whose 16-bit words sum to zero must produce 0xffff.
	if got := checksum([]byte{0xff, 0xff}); got != 0x0000 {
		t.Errorf("checksum(0xffff) = 0x%04x, want 0x0000", got)
	}
	// An odd-length message must include the padded trailing byte.
	odd := checksum([]byte{0x01, 0x02, 0x03})
	even := checksum([]byte{0x01, 0x02, 0x03, 0x00})
	if odd != even {
		t.Errorf("checksum did not pad the odd byte: 0x%04x vs 0x%04x", odd, even)
	}
}

func TestReplySequence(t *testing.T) {
	tests := []struct {
		name string
		msg  []byte
		want uint16
	}{
		{name: "typical", msg: []byte{0, 0, 0, 0, 0, 0, 0x01, 0x2c}, want: 300},
		{name: "zero", msg: []byte{0, 0, 0, 0, 0, 0, 0, 0}, want: 0},
		{name: "too short", msg: []byte{0, 0}, want: 0},
		{name: "nil", msg: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replySequence(tt.msg); got != tt.want {
				t.Errorf("replySequence = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProtoReply(t *testing.T) {
	if got := protoReply(icmpv4EchoRequest); got != icmpv4EchoReply {
		t.Errorf("protoReply(v4 request) = %d, want %d", got, icmpv4EchoReply)
	}
	if got := protoReply(icmpv6EchoRequest); got != icmpv6EchoReply {
		t.Errorf("protoReply(v6 request) = %d, want %d", got, icmpv6EchoReply)
	}
}

func TestICMPNetworkSelectsFamily(t *testing.T) {
	tests := []struct {
		address string
		network string
		proto   uint8
	}{
		{address: "127.0.0.1", network: "ip4:icmp", proto: icmpv4EchoRequest},
		{address: "192.168.1.1", network: "ip4:icmp", proto: icmpv4EchoRequest},
		{address: "::1", network: "ip6:ipv6-icmp", proto: icmpv6EchoRequest},
		{address: "fe80::1", network: "ip6:ipv6-icmp", proto: icmpv6EchoRequest},
		{address: "::ffff:192.168.1.1", network: "ip4:icmp", proto: icmpv4EchoRequest},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			network, proto, err := icmpNetwork(net.ParseIP(tt.address))
			if err != nil {
				t.Fatalf("icmpNetwork returned error: %v", err)
			}
			if network != tt.network {
				t.Errorf("network = %q, want %q", network, tt.network)
			}
			if proto != tt.proto {
				t.Errorf("proto = %d, want %d", proto, tt.proto)
			}
		})
	}
}

func TestICMPNetworkRejectsNonAddress(t *testing.T) {
	if _, _, err := icmpNetwork(nil); err == nil {
		t.Error("icmpNetwork(nil) succeeded, want an error")
	}
}

func TestWildcard(t *testing.T) {
	if got := wildcard(net.ParseIP("127.0.0.1")); got != "0.0.0.0" {
		t.Errorf("wildcard(v4) = %q, want 0.0.0.0", got)
	}
	if got := wildcard(net.ParseIP("::1")); got != "::" {
		t.Errorf("wildcard(v6) = %q, want ::", got)
	}
}

func TestICMPIdentifierIsStableAndVariesWithSequence(t *testing.T) {
	first := icmpIdentifier(1)
	if first != icmpIdentifier(1) {
		t.Error("icmpIdentifier is not stable for the same sequence")
	}
	if first == icmpIdentifier(2) {
		t.Error("icmpIdentifier does not vary with the sequence")
	}
}
