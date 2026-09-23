package ping

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestRunWithPacketErrorCountsAsLost(t *testing.T) {
	boom := errors.New("socket exploded")
	fake := &fakeEchoer{
		durations: []time.Duration{time.Millisecond},
		errs:      []error{nil, boom},
	}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{Count: 2})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}

	packet := report.Result.Packets[1]
	if packet.Status != StatusError {
		t.Errorf("status = %v, want ERROR", packet.Status)
	}
	if !errors.Is(packet.Err, boom) {
		t.Errorf("Err = %v, want the underlying failure preserved", packet.Err)
	}
	if report.Result.Lost() != 1 {
		t.Errorf("Lost() = %d, want 1", report.Result.Lost())
	}
}

func TestRunWithAllPacketsLostKeepsGoing(t *testing.T) {
	// Every packet failing must still produce a complete result: the run is only
	// abandoned when the context ends.
	fake := &fakeEchoer{errs: []error{ErrTimeout, ErrTimeout, ErrTimeout}}

	report, err := RunWith(context.Background(), fake, "127.0.0.1", Options{Count: 3})
	if err != nil {
		t.Fatalf("RunWith returned error: %v", err)
	}
	if report.Result.Sent != 3 || report.Result.Received != 0 {
		t.Errorf("Sent/Received = %d/%d, want 3/0", report.Result.Sent, report.Result.Received)
	}
	if fake.calls != 3 {
		t.Errorf("echo calls = %d, want 3", fake.calls)
	}
}

func TestRunWithRejectsNonPositiveCount(t *testing.T) {
	// A count of zero or less cannot be satisfied; the domain rejects it rather
	// than silently sending nothing or looping forever.
	for _, count := range []int{-1, -100} {
		if _, err := RunWith(context.Background(), &fakeEchoer{}, "127.0.0.1", Options{Count: count}); err == nil {
			t.Errorf("RunWith(Count: %d) succeeded, want an error", count)
		}
	}
}

func TestRunWithResolutionFailureIsAResolveError(t *testing.T) {
	sentinel := &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}
	restore := stubLookup(nil, sentinel)
	defer restore()

	fake := &fakeEchoer{}
	_, err := RunWith(context.Background(), fake, "nope.invalid", Options{Count: 1})
	if err == nil {
		t.Fatal("RunWith returned no error for an unresolvable host")
	}

	var resolveErr *ResolveError
	if !errors.As(err, &resolveErr) {
		t.Fatalf("error %v is not a *ResolveError", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the resolver error preserved", err)
	}
	if resolveErr.Host != "nope.invalid" {
		t.Errorf("Host = %q, want nope.invalid", resolveErr.Host)
	}
	// Nothing may be measured when the address is unknown.
	if fake.calls != 0 {
		t.Errorf("echo calls = %d, want 0 after a resolution failure", fake.calls)
	}
}

func TestRunWithEmptyResolutionIsAnError(t *testing.T) {
	// A resolver that answers with nothing leaves no address to probe.
	restore := stubLookup(nil, nil)
	defer restore()

	if _, err := RunWith(context.Background(), &fakeEchoer{}, "empty.invalid", Options{Count: 1}); err == nil {
		t.Fatal("RunWith succeeded with no resolved addresses")
	}
}

func TestRunWithContextAlreadyCancelledDoesNotProbe(t *testing.T) {
	fake := &fakeEchoer{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunWith(ctx, fake, "127.0.0.1", Options{Count: 3})
	if err == nil {
		t.Fatal("RunWith returned no error for a cancelled context")
	}
	if !IsCanceled(err) {
		t.Errorf("error = %v, want it classified as cancelled", err)
	}
	// No packet may be sent once the caller has cancelled.
	if fake.calls != 0 {
		t.Errorf("echo calls = %d, want 0", fake.calls)
	}
}

func TestRunWithDeadlineIsClassifiedAsTimeout(t *testing.T) {
	fake := &fakeEchoer{block: true}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := RunWith(ctx, fake, "127.0.0.1", Options{Count: 4})
	if err == nil {
		t.Fatal("RunWith returned no error for an expired deadline")
	}
	if !IsTimeout(err) {
		t.Errorf("error = %v, want it classified as a timeout", err)
	}
	if IsCanceled(err) {
		t.Error("IsCanceled = true for a deadline, want false")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want the context sentinel preserved", err)
	}
}
