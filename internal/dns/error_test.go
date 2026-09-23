package dns

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolveWithDeadlineIsClassifiedAsTimeout(t *testing.T) {
	fake := &fakeResolver{delay: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := ResolveWith(ctx, fake, "slow.invalid")
	if err == nil {
		t.Fatal("ResolveWith returned no error for an expired deadline")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("error = %v, want errors.Is(err, ErrTimeout)", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want the context sentinel preserved", err)
	}
	if !IsTimeout(err) {
		t.Error("IsTimeout = false, want true")
	}

	var lookupErr *LookupError
	if !errors.As(err, &lookupErr) {
		t.Fatalf("error %v is not a *LookupError", err)
	}
	if !lookupErr.Timeout {
		t.Error("LookupError.Timeout = false, want true")
	}
	if lookupErr.Canceled {
		t.Error("LookupError.Canceled = true for a deadline, want false")
	}
}

func TestResolveWithCancellationIsClassifiedAsCanceled(t *testing.T) {
	fake := &fakeResolver{delay: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the lookup, exactly like Ctrl-C

	_, err := ResolveWith(ctx, fake, "cancelled.invalid")
	if err == nil {
		t.Fatal("ResolveWith returned no error for a cancelled context")
	}
	if !IsCanceled(err) {
		t.Errorf("error = %v, want errors.Is(err, context.Canceled)", err)
	}
	if IsTimeout(err) {
		t.Error("IsTimeout = true for a cancellation, want false")
	}

	var lookupErr *LookupError
	if !errors.As(err, &lookupErr) {
		t.Fatalf("error %v is not a *LookupError", err)
	}
	if !lookupErr.Canceled {
		t.Error("LookupError.Canceled = false, want true")
	}
	if lookupErr.Timeout {
		t.Error("LookupError.Timeout = true for a cancellation, want false")
	}
}

// TestResolveWithTimeoutIsNotStringMatched proves the classification comes from
// the context sentinels: an ordinary resolver error that merely mentions a
// timeout in its text must not be reported as one.
func TestResolveWithTimeoutIsNotStringMatched(t *testing.T) {
	fake := &fakeResolver{err: errors.New("i/o timeout")}

	_, err := ResolveWith(context.Background(), fake, "misleading.invalid")
	if err == nil {
		t.Fatal("ResolveWith returned no error")
	}
	if IsTimeout(err) {
		t.Errorf("error %v was classified as a timeout from its text alone", err)
	}
}

func TestLookupErrorMessages(t *testing.T) {
	const host = "example.invalid"

	timeout := (&LookupError{Host: host, Timeout: true, Err: context.DeadlineExceeded}).Error()
	if !strings.Contains(timeout, "timed out") {
		t.Errorf("timeout message = %q, want it to describe a timeout", timeout)
	}
	if !strings.Contains(timeout, host) {
		t.Errorf("timeout message = %q, want it to mention the host", timeout)
	}
	if !strings.Contains(timeout, context.DeadlineExceeded.Error()) {
		t.Errorf("timeout message = %q, want it to preserve the cause", timeout)
	}

	canceled := (&LookupError{Host: host, Canceled: true, Err: context.Canceled}).Error()
	if !strings.Contains(canceled, "cancelled") {
		t.Errorf("cancellation message = %q, want it to say cancelled", canceled)
	}

	plain := (&LookupError{Host: host, Err: errors.New("boom")}).Error()
	if !strings.Contains(plain, "failed to resolve") {
		t.Errorf("plain message = %q, want it to describe a resolution failure", plain)
	}
	if !strings.Contains(plain, "boom") {
		t.Errorf("plain message = %q, want it to preserve the underlying error", plain)
	}
}

func TestIsTimeoutAndIsCanceledOnUnrelatedErrors(t *testing.T) {
	plain := errors.New("boom")
	if IsTimeout(plain) {
		t.Error("IsTimeout(plain) = true, want false")
	}
	if IsCanceled(plain) {
		t.Error("IsCanceled(plain) = true, want false")
	}
	if IsTimeout(nil) {
		t.Error("IsTimeout(nil) = true, want false")
	}
	if IsCanceled(nil) {
		t.Error("IsCanceled(nil) = true, want false")
	}
}
