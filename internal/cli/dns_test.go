package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"nselecttrace/internal/dns"
)

// newDNSTestApp returns an App whose DNS lookups never touch the network.
//
// The fake records the hosts and contexts it was called with, so a test can
// assert not only on output but also that resolution was attempted at all, and
// under what deadline.
func newDNSTestApp(fn resolveFunc) (*App, *strings.Builder, *strings.Builder, *fakeDNS) {
	fake := &fakeDNS{fn: fn}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	app := New(stdout, stderr)
	app.resolve = fake.lookup

	return app, stdout, stderr, fake
}

// fakeDNS stands in for the resolver and remembers how it was called.
type fakeDNS struct {
	fn resolveFunc

	hosts       []string
	deadlines   []time.Duration // remaining budget at call time
	hasDeadline []bool
}

func (f *fakeDNS) lookup(ctx context.Context, host string) (dns.Result, error) {
	f.hosts = append(f.hosts, host)

	if deadline, ok := ctx.Deadline(); ok {
		f.deadlines = append(f.deadlines, time.Until(deadline))
		f.hasDeadline = append(f.hasDeadline, true)
	} else {
		f.deadlines = append(f.deadlines, 0)
		f.hasDeadline = append(f.hasDeadline, false)
	}

	if f.fn == nil {
		return dns.Result{Host: host}, nil
	}
	return f.fn(ctx, host)
}

// okResult answers with one A and one AAAA record.
func okResult(ctx context.Context, host string) (dns.Result, error) {
	_ = ctx
	return dns.Result{
		Host: host,
		Records: []dns.Record{
			{Type: dns.TypeA, Address: "93.184.216.34"},
			{Type: dns.TypeAAAA, Address: "2606:2800::1"},
		},
		Duration: 24700 * time.Microsecond,
	}, nil
}

func TestDNSCommandIsRegistered(t *testing.T) {
	app, _, _, _ := newDNSTestApp(nil)

	cmd, ok := app.Command("dns")
	if !ok {
		t.Fatal("dns command is not registered")
	}
	if cmd.Summary == "" {
		t.Error("dns command has no summary")
	}
	if cmd.Usage != "dns <host> [--timeout <duration>]" {
		t.Errorf("Usage = %q, want the documented form", cmd.Usage)
	}
	if len(cmd.Arguments) != 2 {
		t.Errorf("Arguments = %d, want 2 (host and --timeout)", len(cmd.Arguments))
	}
	if cmd.Run == nil {
		t.Fatal("dns command has no Run function")
	}
}

func TestDNSCommandIsListedInHelp(t *testing.T) {
	app, stdout, _, _ := newDNSTestApp(nil)

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "dns") {
		t.Errorf("dns is not listed in help:\n%s", stdout.String())
	}
}

func TestDNSCommandHelpDocumentsTimeout(t *testing.T) {
	app, stdout, _, _ := newDNSTestApp(nil)

	if err := app.Run(context.Background(), []string{"dns", "--help"}); err != nil {
		t.Fatalf("Run(dns --help) returned error: %v", err)
	}
	out := stdout.String()

	for _, want := range []string{
		"nselecttrace dns <host> [--timeout <duration>]",
		"--timeout",
		dns.DefaultTimeout.String(),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dns help is missing %q:\n%s", want, out)
		}
	}
	// The help must not advertise features that do not exist. Only the
	// descriptive text is checked, never the usage line, because the program
	// name itself contains "trace".
	body := out
	if i := strings.Index(body, "Arguments:"); i > 0 {
		body = body[i:]
	}
	lower := strings.ToLower(body)
	for _, unwanted := range []string{"reverse", "ping", "traceroute", "dnssec", "ptr record"} {
		if strings.Contains(lower, unwanted) {
			t.Errorf("dns help mentions %q, which is not implemented:\n%s", unwanted, out)
		}
	}
}

func TestDNSCommandPassesDeadlineDerivedFromTimeout(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{name: "default", args: nil, want: dns.DefaultTimeout},
		{name: "separate form", args: []string{"--timeout", "5s"}, want: 5 * time.Second},
		{name: "joined form", args: []string{"--timeout=250ms"}, want: 250 * time.Millisecond},
		{name: "long", args: []string{"--timeout", "1m"}, want: time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, _, fake := newDNSTestApp(okResult)

			args := append([]string{"dns", "example.com"}, tt.args...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run(%v) returned error: %v", args, err)
			}

			if len(fake.hasDeadline) != 1 || !fake.hasDeadline[0] {
				t.Fatalf("the resolver was called without a deadline: %v", fake.hasDeadline)
			}
			// The remaining budget is slightly below the configured timeout
			// because time passes between setting and observing it.
			got := fake.deadlines[0]
			if got > tt.want {
				t.Errorf("deadline budget = %v, want no more than %v", got, tt.want)
			}
			if got < tt.want/2 {
				t.Errorf("deadline budget = %v, want close to %v", got, tt.want)
			}
		})
	}
}

func TestDNSCommandParentContextStillWins(t *testing.T) {
	// A cancelled parent must abort the lookup even though the command adds a
	// deadline of its own: the child derives from the parent, it never replaces
	// it, so Ctrl-C keeps working.
	app, _, _, fake := newDNSTestApp(func(ctx context.Context, host string) (dns.Result, error) {
		<-ctx.Done()
		return dns.Result{}, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx, []string{"dns", "example.com", "--timeout", "1m"})
	if err == nil {
		t.Fatal("Run returned nil error for a cancelled parent context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled preserved", err)
	}
	if len(fake.hosts) != 1 {
		t.Errorf("resolver calls = %v, want exactly one attempt", fake.hosts)
	}
}

func TestDNSCommandResolverTimeoutIsARuntimeError(t *testing.T) {
	// Exercise the real timeout classification by having the fake delegate to
	// dns.ResolveWith with a resolver that never answers in time.
	app, _, stderr, _ := newDNSTestApp(func(ctx context.Context, host string) (dns.Result, error) {
		return dns.ResolveWith(ctx, slowResolver{}, host)
	})

	err := app.Run(context.Background(), []string{"dns", "slow.invalid", "--timeout", "1ms"})
	if err == nil {
		t.Fatal("Run returned nil error for a timed-out lookup")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1 for a runtime failure", exitErr.Code)
	}
	if !dns.IsTimeout(err) {
		t.Errorf("error = %v, want it to be recognisable as a timeout", err)
	}
	// A runtime failure must not print usage.
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("usage was printed for a runtime failure: %q", stderr.String())
	}
}

// slowResolver never answers before the caller's deadline expires.
type slowResolver struct{}

func (slowResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestDNSCommandResolverFailureIsARuntimeError(t *testing.T) {
	sentinel := errors.New("no such host")
	app, stdout, stderr, _ := newDNSTestApp(func(ctx context.Context, host string) (dns.Result, error) {
		return dns.Result{}, sentinel
	})

	err := app.Run(context.Background(), []string{"dns", "nope.invalid"})
	if err == nil {
		t.Fatal("Run returned nil error for a resolver failure")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the resolver error preserved", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
	// The App prints nothing itself: main.go reports the error once. A runtime
	// failure must never print usage.
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("usage was printed for a runtime failure: %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("a failed resolution produced stdout output: %q", stdout.String())
	}
}

func TestParseDNSArgsAcceptsHostAndTimeout(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantHost    string
		wantTimeout time.Duration
	}{
		{name: "host only", args: []string{"example.com"}, wantHost: "example.com", wantTimeout: dns.DefaultTimeout},
		{name: "timeout after host", args: []string{"example.com", "--timeout", "1s"}, wantHost: "example.com", wantTimeout: time.Second},
		{name: "timeout before host", args: []string{"--timeout", "2s", "example.com"}, wantHost: "example.com", wantTimeout: 2 * time.Second},
		{name: "joined form", args: []string{"example.com", "--timeout=500ms"}, wantHost: "example.com", wantTimeout: 500 * time.Millisecond},
		{name: "ip literal is accepted as a host", args: []string{"1.1.1.1"}, wantHost: "1.1.1.1", wantTimeout: dns.DefaultTimeout},
		{name: "ipv6 literal is accepted as a host", args: []string{"::1"}, wantHost: "::1", wantTimeout: dns.DefaultTimeout},
		{name: "localhost", args: []string{"localhost"}, wantHost: "localhost", wantTimeout: dns.DefaultTimeout},
		{name: "trailing dot", args: []string{"example.com."}, wantHost: "example.com.", wantTimeout: dns.DefaultTimeout},
		{name: "minutes", args: []string{"example.com", "--timeout", "1m"}, wantHost: "example.com", wantTimeout: time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, timeout, err := parseDNSArgs(tt.args)
			if err != nil {
				t.Fatalf("parseDNSArgs(%v) returned error: %v", tt.args, err)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", timeout, tt.wantTimeout)
			}
		})
	}
}

func TestParseDNSArgsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing host", args: nil},
		{name: "only a flag", args: []string{"--timeout", "5s"}},
		{name: "extra positional", args: []string{"example.com", "extra.com"}},
		{name: "extra positional before flag", args: []string{"a.com", "b.com", "--timeout", "5s"}},
		{name: "unknown flag", args: []string{"example.com", "--nope"}},
		{name: "timeout not a duration", args: []string{"example.com", "--timeout", "abc"}},
		{name: "timeout empty", args: []string{"example.com", "--timeout="}},
		{name: "timeout zero", args: []string{"example.com", "--timeout", "0"}},
		{name: "timeout negative", args: []string{"example.com", "--timeout", "-1s"}},
		{name: "timeout missing value", args: []string{"example.com", "--timeout"}},
		{name: "timeout without unit", args: []string{"example.com", "--timeout", "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseDNSArgs(tt.args)
			if err == nil {
				t.Fatalf("parseDNSArgs(%v) succeeded, want an error", tt.args)
			}
			if !errors.Is(err, ErrUsage) {
				t.Errorf("error = %v, want it to wrap ErrUsage", err)
			}
		})
	}
}

func TestDNSCommandRejectsBadInputWithExitCode2AndNoLookup(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing host", args: []string{"dns"}},
		{name: "extra positional", args: []string{"dns", "example.com", "extra"}},
		{name: "unknown flag", args: []string{"dns", "example.com", "--nope"}},
		{name: "invalid timeout", args: []string{"dns", "example.com", "--timeout", "abc"}},
		{name: "zero timeout", args: []string{"dns", "example.com", "--timeout", "0"}},
		{name: "negative timeout", args: []string{"dns", "example.com", "--timeout", "-1s"}},
		{name: "empty timeout", args: []string{"dns", "example.com", "--timeout="}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, stderr, fake := newDNSTestApp(okResult)

			err := app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatalf("Run(%v) returned nil error", tt.args)
			}

			var exitErr *ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("error %v is not an *ExitError", err)
			}
			if exitErr.Code != 2 {
				t.Errorf("exit code = %d, want 2", exitErr.Code)
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Errorf("usage was not written to stderr: %q", stderr.String())
			}
			// The whole point of validating first: no lookup is attempted.
			if len(fake.hosts) != 0 {
				t.Errorf("resolver was called %v times despite invalid input", len(fake.hosts))
			}
		})
	}
}

func TestDNSCommandReportsWriteFailure(t *testing.T) {
	app := New(failingWriter{}, io.Discard)
	app.resolve = okResult

	err := app.Run(context.Background(), []string{"dns", "example.com"})
	if err == nil {
		t.Fatal("Run returned nil error for a failing writer")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("error = %q, want a write failure", err.Error())
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
}

func TestDNSCommandDefaultResolverIsTheRealOne(t *testing.T) {
	// A production App must reach the real resolver, not a nil function.
	app, _, _, _ := newDNSTestApp(nil)
	app.resolve = nil

	if app.resolver() == nil {
		t.Fatal("resolver() returned nil for a production App")
	}
}

func TestDNSCommandEmptyAnswerIsSuccess(t *testing.T) {
	app, stdout, _, _ := newDNSTestApp(func(ctx context.Context, host string) (dns.Result, error) {
		return dns.Result{Host: host, Duration: time.Millisecond}, nil
	})

	if err := app.Run(context.Background(), []string{"dns", "empty.invalid"}); err != nil {
		t.Fatalf("Run returned error for an empty answer: %v", err)
	}
	if !strings.Contains(stdout.String(), "No records found") {
		t.Errorf("an empty answer is not reported honestly:\n%s", stdout.String())
	}
}
