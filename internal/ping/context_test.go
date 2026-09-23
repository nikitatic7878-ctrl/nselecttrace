package ping

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// TestRunWithCancellationMidRunStopsAndIsNotPacketLoss verifies that stopping
// the run is reported as a cancellation rather than as a lost packet.
//
// The fake blocks until the context ends, and cancellation is triggered only
// after the first Echo has been entered, so the test is deterministic and needs
// no sleep race.
func TestRunWithCancellationMidRunStopsAndIsNotPacketLoss(t *testing.T) {
	fake := &fakeEchoer{block: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entered := make(chan struct{})
	go func() {
		for fake.calls == 0 {
			time.Sleep(time.Millisecond)
		}
		close(entered)
		cancel()
	}()

	_, err := RunWith(ctx, fake, "127.0.0.1", Options{Count: 4})
	<-entered

	if err == nil {
		t.Fatal("RunWith returned no error for a cancelled run")
	}
	if !IsCanceled(err) {
		t.Errorf("error = %v, want it classified as cancelled", err)
	}
	if IsTimeout(err) {
		t.Error("IsTimeout = true for a cancellation, want false")
	}

	var runErr *RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("error %v is not a *RunError", err)
	}
	if !runErr.Canceled {
		t.Error("RunError.Canceled = false, want true")
	}
	// The run must stop after cancelling instead of sending the remaining
	// requests.
	if fake.calls != 1 {
		t.Errorf("echo calls = %d, want 1", fake.calls)
	}
}

func TestResolveOnePrefersIPv4ThenLowest(t *testing.T) {
	tests := []struct {
		name  string
		addrs []net.IPAddr
		want  string
	}{
		{
			name:  "ipv4 preferred over ipv6",
			addrs: []net.IPAddr{ipAddr("2606:2800::1"), ipAddr("93.184.216.34")},
			want:  "93.184.216.34",
		},
		{
			name:  "lowest ipv4 wins regardless of order",
			addrs: []net.IPAddr{ipAddr("192.168.1.5"), ipAddr("10.0.0.1"), ipAddr("172.16.0.1")},
			want:  "10.0.0.1",
		},
		{
			name:  "ipv6 used when there is no ipv4",
			addrs: []net.IPAddr{ipAddr("fe80::1"), ipAddr("::1")},
			want:  "::1",
		},
		{
			name:  "empty addresses are skipped",
			addrs: []net.IPAddr{{IP: nil}, {IP: net.IP{}}, ipAddr("10.0.0.2")},
			want:  "10.0.0.2",
		},
		{
			name:  "ipv4 mapped address counts as ipv4",
			addrs: []net.IPAddr{ipAddr("::ffff:192.168.1.1"), ipAddr("fe80::1")},
			want:  "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := stubLookup(tt.addrs, nil)
			defer restore()

			ip, err := resolveOne(context.Background(), "example.com")
			if err != nil {
				t.Fatalf("resolveOne returned error: %v", err)
			}
			if ip.String() != tt.want {
				t.Errorf("resolveOne = %q, want %q", ip.String(), tt.want)
			}
		})
	}
}

func TestResolveOneAcceptsLiteralAddressWithoutLookup(t *testing.T) {
	// A literal must not be sent to the resolver, so resolution cannot fail.
	lookups := 0
	prev := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		lookups++
		return nil, errors.New("must not be called")
	}
	defer func() { lookupIPAddr = prev }()

	for _, literal := range []string{"127.0.0.1", "::1", "2606:2800::1"} {
		ip, err := resolveOne(context.Background(), literal)
		if err != nil {
			t.Fatalf("resolveOne(%q) returned error: %v", literal, err)
		}
		if ip.String() != literal {
			t.Errorf("resolveOne(%q) = %q", literal, ip.String())
		}
	}
	if lookups != 0 {
		t.Errorf("resolver was called %d times for literal addresses, want 0", lookups)
	}
}

func TestResolveOnePropagatesCancellation(t *testing.T) {
	restore := stubLookup(nil, context.Canceled)
	defer restore()

	_, err := resolveOne(context.Background(), "anything.invalid")
	if err == nil {
		t.Fatal("resolveOne returned no error")
	}
	if !IsCanceled(err) {
		t.Errorf("error = %v, want it classified as cancelled", err)
	}
}

func TestLowestIsDeterministic(t *testing.T) {
	ips := []net.IP{net.ParseIP("192.168.1.5"), net.ParseIP("10.0.0.1"), net.ParseIP("172.16.0.1")}

	first := lowest(ips)
	second := lowest(ips)

	if first.String() != second.String() {
		t.Fatalf("lowest is not deterministic: %v vs %v", first, second)
	}
	if first.String() != "10.0.0.1" {
		t.Errorf("lowest = %v, want 10.0.0.1", first)
	}
}
