package dns

import (
	"context"
	"errors"
	"fmt"
)

// ErrTimeout reports that the resolution did not finish within the caller's
// deadline. It is exported so the CLI can present it as a timeout without
// inspecting error strings, and it stays wrapped around the original error so
// the chain is never lost.
var ErrTimeout = errors.New("DNS lookup timed out")

// LookupError describes a failed forward resolution.
//
// It exists to keep three things apart that a user needs to tell apart, without
// any string matching at the call site: a deadline, a cancellation, and a plain
// resolver failure (name not found, temporary failure, no network).
type LookupError struct {
	// Host is the name that was looked up.
	Host string
	// Timeout is true when the caller's deadline expired.
	Timeout bool
	// Canceled is true when the context was cancelled, for example by Ctrl-C.
	Canceled bool
	// Err is the underlying error from the resolver or the context.
	Err error
}

func (e *LookupError) Error() string {
	switch {
	case e.Timeout:
		return fmt.Sprintf("%s for %q: %v", ErrTimeout, e.Host, e.Err)
	case e.Canceled:
		return fmt.Sprintf("DNS lookup of %q was cancelled: %v", e.Host, e.Err)
	default:
		return fmt.Sprintf("failed to resolve %q: %v", e.Host, e.Err)
	}
}

// Unwrap reports the underlying error so that errors.Is keeps working for both
// context sentinels, and so that callers can detect a timeout with either
// errors.Is(err, ErrTimeout) or errors.Is(err, context.DeadlineExceeded).
func (e *LookupError) Unwrap() error {
	if e.Timeout {
		return errors.Join(ErrTimeout, e.Err)
	}
	return e.Err
}

// wrapLookupError classifies a resolver failure.
//
// Classification uses only the context sentinels, never the error text: string
// matching on resolver messages is fragile across platforms and Go versions.
func wrapLookupError(host string, err error) error {
	return &LookupError{
		Host:     host,
		Timeout:  errors.Is(err, context.DeadlineExceeded),
		Canceled: errors.Is(err, context.Canceled),
		Err:      err,
	}
}

// IsTimeout reports whether err is a lookup that ran out of time.
//
// It is a convenience over errors.Is(err, ErrTimeout) for callers that want the
// distinction without importing the sentinel.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}

// IsCanceled reports whether err is a lookup that was cancelled rather than
// failed. A cancellation is not an error condition the user caused by typing,
// so the CLI reports it quietly.
func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
