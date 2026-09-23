// Package ping measures ICMP echo reachability of a host.
//
// It resolves the host, sends one echo request per measurement and reports the
// round-trip time of each reply. The package returns a plain domain model so
// that the CLI, a future TUI and JSON output can all consume the same values,
// and it reaches the network through a small seam so that every test runs
// without ICMP, without DNS and without elevated privileges.
//
// This is a network operation, so Run takes a context.Context and honours
// cancellation.
package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// DefaultTimeout bounds the whole operation, not a single echo request. It is
// the same default as `nselecttrace dns`, so a user only has to learn one
// number.
const DefaultTimeout = 5 * time.Second

// DefaultCount is how many echo requests are sent when the caller does not
// choose. Four matches the convention of the platform ping tools without
// copying their output format.
const DefaultCount = 4

// payloadSize is the ICMP payload, giving the conventional 64-byte request
// (8-byte ICMP header + 56-byte payload).
const payloadSize = 56

// Status describes the outcome of a single echo request.
type Status uint8

// Packet statuses.
const (
	// StatusOK means a matching echo reply arrived.
	StatusOK Status = iota + 1
	// StatusTimeout means no reply arrived before the per-packet deadline.
	StatusTimeout
	// StatusError means the request or the read failed.
	StatusError
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusTimeout:
		return "TIMEOUT"
	case StatusError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Packet is the outcome of one echo request.
//
// Duration is meaningful only when Status is StatusOK: a timeout or a failure
// has no round-trip time, and inventing one would misreport the measurement.
type Packet struct {
	Sequence int
	Status   Status
	Duration time.Duration
	// Err carries the failure reason for StatusError; it is nil otherwise.
	Err error
}

// Succeeded reports whether the packet measured a round trip.
func (p Packet) Succeeded() bool { return p.Status == StatusOK }

// Result is the outcome of one ping run.
type Result struct {
	// Host is the name or address the caller asked for.
	Host string
	// Address is the single address that was actually probed, chosen
	// deterministically from the resolved set.
	Address string
	// Packets holds one entry per echo request, in sequence order.
	Packets []Packet
	// Sent is how many echo requests were attempted.
	Sent int
	// Received is how many produced an echo reply.
	Received int
}

// Lost reports how many echo requests produced no reply.
//
// A status of StatusError counts as lost as well: from the caller's point of
// view the packet did not come back.
func (r Result) Lost() int { return r.Sent - r.Received }

// Report is the full outcome of a run.
type Report struct {
	Result Result
	// Duration is how long the whole run took, measured around the operation
	// and excluding argument parsing and rendering.
	Duration time.Duration
}

// Options configures a run.
type Options struct {
	// Count is how many echo requests to send. Zero means DefaultCount.
	Count int
}

// normalise applies defaults and rejects unusable values.
func (o Options) normalise() (Options, error) {
	if o.Count == 0 {
		o.Count = DefaultCount
	}
	if o.Count < 1 {
		return o, fmt.Errorf("ping count must be at least 1, got %d", o.Count)
	}
	return o, nil
}

// echoer is the seam between this package and ICMP.
//
// One call sends one echo request for the given sequence number and returns the
// round-trip time of the matching reply. It is unexported so that no caller
// depends on a concrete transport; Run and RunWith are the API, and tests
// substitute a fake.
type echoer interface {
	// Echo sends one request to ip and waits for the reply.
	//
	// A reply that does not arrive in time is reported as ErrTimeout.
	Echo(ctx context.Context, ip net.IP, sequence int) (time.Duration, error)
}

// RunWith runs the measurement through the supplied transport.
//
// It is exported so that the whole flow can be exercised without ICMP. e must
// not be nil.
func RunWith(ctx context.Context, e echoer, host string, opts Options) (Report, error) {
	opts, err := opts.normalise()
	if err != nil {
		return Report{}, err
	}

	start := time.Now()

	ip, err := resolveOne(ctx, host)
	if err != nil {
		return Report{}, err
	}

	result := Result{
		Host:    host,
		Address: ip.String(),
		Packets: make([]Packet, 0, opts.Count),
	}

	for sequence := 1; sequence <= opts.Count; sequence++ {
		// A cancelled or expired parent must stop the run before sending
		// another packet, so the failure is attributed to the context and not
		// reported as a lost packet.
		if err := ctx.Err(); err != nil {
			return Report{}, contextError(host, err)
		}

		result.Sent++

		duration, echoErr := e.Echo(ctx, ip, sequence)
		packet := Packet{Sequence: sequence}

		switch {
		case echoErr == nil:
			packet.Status = StatusOK
			packet.Duration = duration
			result.Received++
		case errors.Is(echoErr, ErrTimeout):
			packet.Status = StatusTimeout
		case errors.Is(echoErr, context.Canceled), errors.Is(echoErr, context.DeadlineExceeded):
			// The caller asked to stop, or the run's budget ran out: neither is
			// a lost packet, so the partial result is not reported as one.
			return Report{}, contextError(host, echoErr)
		default:
			packet.Status = StatusError
			packet.Err = echoErr
		}

		result.Packets = append(result.Packets, packet)
	}

	return Report{Result: result, Duration: time.Since(start)}, nil
}

// resolveOne turns host into the single address that will be probed.
//
// A host name may resolve to several addresses. Only one is used, chosen
// deterministically: IPv4 first, then IPv6, and within a family the lowest
// address by textual order. Probing every address at once would make the
// summary ambiguous.
func resolveOne(ctx context.Context, host string) (net.IP, error) {
	// A literal address needs no lookup and cannot fail to resolve.
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}

	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, &ResolveError{
			Host:     host,
			Timeout:  errors.Is(err, context.DeadlineExceeded),
			Canceled: errors.Is(err, context.Canceled),
			Err:      err,
		}
	}

	var v4, v6 []net.IP
	for _, addr := range addrs {
		if len(addr.IP) == 0 {
			continue
		}
		if v4ip := addr.IP.To4(); v4ip != nil {
			v4 = append(v4, v4ip)
			continue
		}
		v6 = append(v6, addr.IP)
	}

	switch {
	case len(v4) > 0:
		return lowest(v4), nil
	case len(v6) > 0:
		return lowest(v6), nil
	default:
		return nil, &ResolveError{Host: host, Err: errors.New("no usable addresses found")}
	}
}

// lowest returns the lexicographically smallest address, a simple deterministic
// choice that does not depend on resolver order.
func lowest(ips []net.IP) net.IP {
	best := ips[0]
	for _, ip := range ips[1:] {
		if ip.String() < best.String() {
			best = ip
		}
	}
	return best
}

// lookupIPAddr is the resolver seam, kept as a variable so resolution can be
// faked independently of ICMP.
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// ResolveError describes a failed address resolution.
type ResolveError struct {
	Host     string
	Timeout  bool
	Canceled bool
	Err      error
}

func (e *ResolveError) Error() string {
	switch {
	case e.Timeout:
		return fmt.Sprintf("resolving %q timed out: %v", e.Host, e.Err)
	case e.Canceled:
		return fmt.Sprintf("resolving %q was cancelled: %v", e.Host, e.Err)
	default:
		return fmt.Sprintf("failed to resolve %q: %v", e.Host, e.Err)
	}
}

// Unwrap reports the underlying error so errors.Is/errors.As keep working.
func (e *ResolveError) Unwrap() error { return e.Err }

// contextError classifies a context failure so the CLI can tell a timeout from
// a cancellation without inspecting error text.
func contextError(host string, err error) error {
	return &RunError{
		Host:     host,
		Timeout:  errors.Is(err, context.DeadlineExceeded),
		Canceled: errors.Is(err, context.Canceled),
		Err:      err,
	}
}

// RunError describes a run that stopped because its context ended.
type RunError struct {
	Host     string
	Timeout  bool
	Canceled bool
	Err      error
}

func (e *RunError) Error() string {
	switch {
	case e.Timeout:
		return fmt.Sprintf("ping to %q timed out: %v", e.Host, e.Err)
	case e.Canceled:
		return fmt.Sprintf("ping to %q was cancelled: %v", e.Host, e.Err)
	default:
		return fmt.Sprintf("ping to %q failed: %v", e.Host, e.Err)
	}
}

// Unwrap reports the underlying error so errors.Is(err, context.Canceled) and
// errors.Is(err, context.DeadlineExceeded) both keep working.
func (e *RunError) Unwrap() error { return e.Err }

// IsTimeout reports whether err describes a run that ran out of time.
func IsTimeout(err error) bool {
	var runErr *RunError
	if errors.As(err, &runErr) {
		return runErr.Timeout
	}
	var resolveErr *ResolveError
	if errors.As(err, &resolveErr) {
		return resolveErr.Timeout
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// IsCanceled reports whether err describes a run that was cancelled.
func IsCanceled(err error) bool {
	var runErr *RunError
	if errors.As(err, &runErr) {
		return runErr.Canceled
	}
	var resolveErr *ResolveError
	if errors.As(err, &resolveErr) {
		return resolveErr.Canceled
	}
	return errors.Is(err, context.Canceled)
}

// ErrTimeout reports that an echo request received no reply in time.
var ErrTimeout = errors.New("no ICMP reply")

// ErrUnsupported reports that ICMP is not available on the current platform.
var ErrUnsupported = errors.New("ICMP is not implemented on this platform")

// defaultEchoer is the real transport used by Run.
var defaultEchoer echoer = &icmpEchoer{}

// Run resolves host, probes one address the given number of times and returns
// the measurements.
//
// The context bounds the whole run and is passed to every network operation, so
// a cancelled parent — Ctrl-C through signal.NotifyContext — stops the run
// between packets and aborts a pending read. This function sets no deadline of
// its own: timeout policy belongs to the caller.
func Run(ctx context.Context, host string, opts Options) (Report, error) {
	return RunWith(ctx, defaultEchoer, host, opts)
}
