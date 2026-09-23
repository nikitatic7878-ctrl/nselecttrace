package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// icmpEchoer sends and receives ICMP echo requests.
//
// It uses the standard library only. net.ListenPacket with "ip4:icmp" gives a
// datagram socket that carries the raw ICMP header, which is what the checksum
// and the echo identifier in the request need. On Windows this works as an
// unprivileged user; on Linux it needs either root or a ping_group_range that
// covers the user, and net.ListenPacket reports the permission failure, which is
// surfaced honestly rather than masked.
//
// A socket is opened per connection attempt and closed immediately. Opening it
// once up front would be marginally faster, but ping reuses one socket across
// packets and a stale ICMP error on that socket can be mistaken for the reply to
// the packet that is currently in flight, which would corrupt the measurement.
// This is a diagnostic command, so correctness beats a few microseconds.
type icmpEchoer struct{}

// Echo sends one echo request to ip and waits for its reply.
func (e *icmpEchoer) Echo(ctx context.Context, ip net.IP, sequence int) (time.Duration, error) {
	network, proto, err := icmpNetwork(ip)
	if err != nil {
		return 0, err
	}

	// The listener is bound to the wildcard address of the right family.
	conn, err := net.ListenPacket(network, wildcard(ip))
	if err != nil {
		return 0, fmt.Errorf("open ICMP socket (%s): %w", network, err)
	}
	defer closeConn(conn)

	// A per-packet deadline derived from the caller's context, so Ctrl-C still
	// wins and a stalled reply cannot outlive the run's budget.
	deadline := time.Now().Add(packetWait)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, fmt.Errorf("set ICMP deadline: %w", err)
	}

	request := buildEchoRequest(proto, icmpIdentifier(sequence), sequence)

	start := time.Now()
	if _, err := conn.WriteTo(request, &net.IPAddr{IP: ip}); err != nil {
		return 0, fmt.Errorf("send ICMP echo: %w", err)
	}

	if err := waitForReply(ctx, conn, ip, proto, sequence); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// packetWait bounds a single echo request. It is deliberately shorter than the
// default run timeout so that one unanswered packet produces a TIMEOUT row and
// the run still continues to the next packet.
const packetWait = 2 * time.Second

// icmpNetwork reports the socket network and the ICMP echo type prefix for ip.
func icmpNetwork(ip net.IP) (network string, proto uint8, err error) {
	if ip4 := ip.To4(); ip4 != nil {
		return "ip4:icmp", icmpv4EchoRequest, nil
	}
	if ip.To16() != nil {
		return "ip6:ipv6-icmp", icmpv6EchoRequest, nil
	}
	return "", 0, fmt.Errorf("not an IP address: %v", ip)
}

// wildcard returns the wildcard bind address for the family of ip.
func wildcard(ip net.IP) string {
	if ip.To4() != nil {
		return "0.0.0.0"
	}
	return "::"
}

// icmpIdentifier derives the echo identifier. The process ID is the convention,
// truncated to the 16 bits the header provides, so replies to a different
// process are not mistaken for ours.
func icmpIdentifier(sequence int) uint16 {
	return uint16(os.Getpid()&0xffff) ^ uint16(sequence)
}

// waitForReply reads until a matching echo reply arrives, the context ends, or
// the socket deadline expires.
func waitForReply(ctx context.Context, conn net.PacketConn, ip net.IP, proto uint8, sequence int) error {
	buf := make([]byte, 1500)

	for {
		// A cancellation between reads must be observed even if the socket
		// deadline is later, so the context is checked first.
		if err := ctx.Err(); err != nil {
			return err
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return ErrTimeout
			}
			return fmt.Errorf("read ICMP reply: %w", err)
		}
		if n < icmpHeaderSize {
			continue
		}

		// The kernel hands back the ICMP message itself, so the header is at
		// offset 0 for both families.
		if buf[0] != protoReply(proto) {
			// Another ICMP message (unreachable, time exceeded, or a reply to
			// an earlier packet) is not our answer; keep reading until the
			// deadline.
			continue
		}
		if replySequence(buf[:n]) != uint16(sequence) {
			continue
		}
		return nil
	}
}

// closeConn discards the close error: the command is about to exit and a failed
// close cannot change the measurement.
func closeConn(conn net.PacketConn) {
	_ = conn.Close()
}
