package ping

import (
	"context"
	"net"
	"testing"
	"time"
)

// fakeEchoer stands in for ICMP. Every domain test uses it, so no test needs a
// raw socket, elevated privileges, real DNS or a network.
type fakeEchoer struct {
	// durations and errs are consumed per sequence number, in order.
	durations []time.Duration
	errs      []error

	// calls counts every request, which lets a test assert that the loop ran
	// the expected number of times or stopped early.
	calls int
	ips   []net.IP

	// block, when set, makes Echo wait for cancellation instead of returning.
	block bool
}

func (f *fakeEchoer) Echo(ctx context.Context, ip net.IP, sequence int) (time.Duration, error) {
	f.calls++
	f.ips = append(f.ips, ip)

	if f.block {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	index := sequence - 1
	if index < len(f.errs) && f.errs[index] != nil {
		return 0, f.errs[index]
	}
	if index < len(f.durations) {
		return f.durations[index], nil
	}
	return time.Millisecond, nil
}

// stubLookup replaces address resolution for the duration of a test.
func stubLookup(addrs []net.IPAddr, err error) func() {
	prev := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if err != nil {
			return nil, err
		}
		return addrs, nil
	}
	return func() { lookupIPAddr = prev }
}

func ipAddr(s string) net.IPAddr { return net.IPAddr{IP: net.ParseIP(s)} }

func TestRunWithSuccessfulPackets(t *testing.T) {
	fake := &fakeEchoer{
		durations: []time.Duration{82 * time.Microsecond, 71 * time.Microsecond, 69 * time.Microsecond},
	}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{Count: 3})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	r := report.Result
	if r.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want 127.0.0.1", r.Host)
	}
	if r.Address != "127.0.0.1" {
		t.Errorf("Address = %q, want 127.0.0.1", r.Address)
	}
	if r.Sent != 3 || r.Received != 3 {
		t.Errorf("Sent/Received = %d/%d, want 3/3", r.Sent, r.Received)
	}
	if r.Lost() != 0 {
		t.Errorf("Lost() = %d, want 0", r.Lost())
	}
	if len(r.Packets) != 3 {
		t.Fatalf("len(Packets) = %d, want 3", len(r.Packets))
	}
	for i, packet := range r.Packets {
		if packet.Sequence != i+1 {
			t.Errorf("packet %d sequence = %d, want %d", i, packet.Sequence, i+1)
		}
		if packet.Status != StatusOK {
			t.Errorf("packet %d status = %v, want OK", i, packet.Status)
		}
		if packet.Duration != fake.durations[i] {
			t.Errorf("packet %d duration = %v, want %v", i, packet.Duration, fake.durations[i])
		}
	}
	if report.Duration < 0 {
		t.Errorf("Duration = %v, want non-negative", report.Duration)
	}
}

func TestRunWithDefaultCount(t *testing.T) {
	fake := &fakeEchoer{}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}
	if report.Result.Sent != DefaultCount {
		t.Errorf("Sent = %d, want the default %d", report.Result.Sent, DefaultCount)
	}
	if fake.calls != DefaultCount {
		t.Errorf("echo calls = %d, want %d", fake.calls, DefaultCount)
	}
}

func TestRunWithTimeoutPacketsAreNotMeasured(t *testing.T) {
	fake := &fakeEchoer{errs: []error{ErrTimeout, ErrTimeout}}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{Count: 2})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	for i, packet := range report.Result.Packets {
		if packet.Status != StatusTimeout {
			t.Errorf("packet %d status = %v, want TIMEOUT", i, packet.Status)
		}
		// A timed-out packet must not carry a fabricated duration.
		if packet.Duration != 0 {
			t.Errorf("packet %d duration = %v, want 0 (not measured)", i, packet.Duration)
		}
		if packet.Succeeded() {
			t.Errorf("packet %d reports Succeeded() = true", i)
		}
	}
	if report.Result.Received != 0 {
		t.Errorf("Received = %d, want 0", report.Result.Received)
	}
	if report.Result.Lost() != 2 {
		t.Errorf("Lost() = %d, want 2", report.Result.Lost())
	}
}

func TestRunWithPartialLoss(t *testing.T) {
	fake := &fakeEchoer{
		durations: []time.Duration{time.Millisecond, 0, time.Millisecond, 0},
		errs:      []error{nil, ErrTimeout, nil, ErrTimeout},
	}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{Count: 4})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	if report.Result.Sent != 4 || report.Result.Received != 2 {
		t.Errorf("Sent/Received = %d/%d, want 4/2", report.Result.Sent, report.Result.Received)
	}
	if report.Result.Lost() != 2 {
		t.Errorf("Lost() = %d, want 2", report.Result.Lost())
	}
	// A partial loss is a normal result, not a failure.
	if report.Result.Packets[1].Status != StatusTimeout {
		t.Errorf("packet 2 status = %v, want TIMEOUT", report.Result.Packets[1].Status)
	}
}
