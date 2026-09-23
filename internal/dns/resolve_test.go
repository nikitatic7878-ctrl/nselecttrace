package dns

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestResolveWithUsesResolverAndReturnsResult(t *testing.T) {
	fake := &fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1"), ipAddr("::1")}}

	result, err := ResolveWith(context.Background(), fake, "localhost")
	if err != nil {
		t.Fatalf("ResolveWith returned error: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "localhost" {
		t.Errorf("resolver calls = %v, want one call for localhost", fake.calls)
	}
	if result.Host != "localhost" {
		t.Errorf("Host = %q, want localhost", result.Host)
	}
	assertRecords(t, result.Records, []Record{{TypeA, "127.0.0.1"}, {TypeAAAA, "::1"}})
}

func TestResolveDurationMeasuresTheOperation(t *testing.T) {
	delay := 20 * time.Millisecond
	fake := &fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}, delay: delay}

	result, err := ResolveWith(context.Background(), fake, "localhost")
	if err != nil {
		t.Fatalf("ResolveWith returned error: %v", err)
	}
	if result.Duration < delay {
		t.Errorf("Duration = %v, want at least the resolver delay %v", result.Duration, delay)
	}
	if result.Duration < 0 {
		t.Errorf("Duration = %v, want non-negative", result.Duration)
	}
}

func TestResolveWithPropagatesResolverError(t *testing.T) {
	sentinel := &net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true}
	fake := &fakeResolver{err: sentinel}

	result, err := ResolveWith(context.Background(), fake, "nope.invalid")
	if err == nil {
		t.Fatalf("ResolveWith = %+v, want an error", result)
	}
	// A failed lookup must not look like an empty successful one.
	if result.Host != "" || len(result.Records) != 0 {
		t.Errorf("Result = %+v, want the zero Result on failure", result)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the resolver error", err)
	}

	var lookupErr *LookupError
	if !errors.As(err, &lookupErr) {
		t.Fatalf("error %v is not a *LookupError", err)
	}
	if lookupErr.Timeout || lookupErr.Canceled {
		t.Errorf("plain resolver failure classified as timeout=%v canceled=%v",
			lookupErr.Timeout, lookupErr.Canceled)
	}
	if lookupErr.Host != "nope.invalid" {
		t.Errorf("Host = %q, want nope.invalid", lookupErr.Host)
	}
}

func TestResolveWithEmptyAnswerIsNotAnError(t *testing.T) {
	// A successful resolution that yields nothing is a real answer.
	fake := &fakeResolver{addrs: nil}

	result, err := ResolveWith(context.Background(), fake, "empty.invalid")
	if err != nil {
		t.Fatalf("ResolveWith returned error: %v", err)
	}
	if !result.Empty() {
		t.Errorf("Empty() = false, want true for %+v", result)
	}
	if result.Host != "empty.invalid" {
		t.Errorf("Host = %q, want empty.invalid", result.Host)
	}
}

func TestDefaultTimeoutIsReasonable(t *testing.T) {
	// A default of zero would look like an instant timeout; the CLI relies on
	// this being a usable budget.
	if DefaultTimeout <= 0 {
		t.Errorf("DefaultTimeout = %v, want a positive duration", DefaultTimeout)
	}
}

func TestResolveSeamDefaultsToTheSystemResolver(t *testing.T) {
	// The package must be wired to a real resolver by default; Resolve is the
	// only function that uses it, and it must not be nil.
	if defaultResolver == nil {
		t.Fatal("defaultResolver is nil")
	}
}
