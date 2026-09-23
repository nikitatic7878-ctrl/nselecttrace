package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"nselecttrace/internal/ping"
)

// newPingTestApp returns an App whose ping never touches the network.
//
// The fake records the host, options and context deadline it was called with, so
// a test can assert that measurement was attempted, with which parameters, and
// under what budget.
func newPingTestApp(fn pingFunc) (*App, *strings.Builder, *strings.Builder, *fakePing) {
	fake := &fakePing{fn: fn}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	app := New(stdout, stderr)
	app.ping = fake.run

	return app, stdout, stderr, fake
}

// fakePing stands in for the measurement and remembers how it was called.
type fakePing struct {
	fn pingFunc

	hosts       []string
	options     []ping.Options
	deadlines   []time.Duration
	hasDeadline []bool
}

func (f *fakePing) run(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
	f.hosts = append(f.hosts, host)
	f.options = append(f.options, opts)

	if deadline, ok := ctx.Deadline(); ok {
		f.deadlines = append(f.deadlines, time.Until(deadline))
		f.hasDeadline = append(f.hasDeadline, true)
	} else {
		f.deadlines = append(f.deadlines, 0)
		f.hasDeadline = append(f.hasDeadline, false)
	}

	if f.fn == nil {
		return okReport(host, opts.Count), nil
	}
	return f.fn(ctx, host, opts)
}

// okReport builds a report with every packet answered.
func okReport(host string, count int) ping.Report {
	if count == 0 {
		count = ping.DefaultCount
	}
	packets := make([]ping.Packet, 0, count)
	for i := 1; i <= count; i++ {
		packets = append(packets, ping.Packet{Sequence: i, Status: ping.StatusOK, Duration: 82 * time.Microsecond})
	}
	return ping.Report{
		Result: ping.Result{
			Host:     host,
			Address:  "127.0.0.1",
			Packets:  packets,
			Sent:     count,
			Received: count,
		},
		Duration: 5 * time.Millisecond,
	}
}

func TestPingCommandIsRegistered(t *testing.T) {
	app, _, _, _ := newPingTestApp(nil)

	cmd, ok := app.Command("ping")
	if !ok {
		t.Fatal("ping command is not registered")
	}
	if cmd.Summary == "" {
		t.Error("ping command has no summary")
	}
	if cmd.Usage != "ping <host> [--count <n>] [--timeout <duration>]" {
		t.Errorf("Usage = %q, want the documented form", cmd.Usage)
	}
	if len(cmd.Arguments) != 3 {
		t.Errorf("Arguments = %d, want 3 (host, --count, --timeout)", len(cmd.Arguments))
	}
	if cmd.Run == nil {
		t.Fatal("ping command has no Run function")
	}
}

func TestPingCommandIsListedInHelp(t *testing.T) {
	app, stdout, _, _ := newPingTestApp(nil)

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ping") {
		t.Errorf("ping is not listed in help:\n%s", stdout.String())
	}
}

func TestPingCommandHelpDocumentsFlags(t *testing.T) {
	app, stdout, _, _ := newPingTestApp(nil)

	if err := app.Run(context.Background(), []string{"ping", "--help"}); err != nil {
		t.Fatalf("Run(ping --help) returned error: %v", err)
	}
	out := stdout.String()

	for _, want := range []string{
		"nselecttrace ping <host> [--count <n>] [--timeout <duration>]",
		"--count",
		"--timeout",
		ping.DefaultTimeout.String(),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ping help is missing %q:\n%s", want, out)
		}
	}
	// The help must not advertise features that do not exist. Only the
	// descriptive text is checked, because the program name itself contains
	// "trace".
	body := out
	if i := strings.Index(body, "Arguments:"); i > 0 {
		body = body[i:]
	}
	lower := strings.ToLower(body)
	for _, unwanted := range []string{"watch", "continuous", "traceroute", "json", "color"} {
		if strings.Contains(lower, unwanted) {
			t.Errorf("ping help mentions %q, which is not implemented:\n%s", unwanted, out)
		}
	}
}

func TestPingCommandRuns(t *testing.T) {
	app, stdout, stderr, fake := newPingTestApp(nil)

	if err := app.Run(context.Background(), []string{"ping", "localhost"}); err != nil {
		t.Fatalf("Run(ping localhost) returned error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"HOST", "ADDRESS", "SEQ", "STATUS", "TIME", "localhost", "127.0.0.1", "OK", "Sent", "Received", "Lost"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if stderr.String() != "" {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}
	if len(fake.hosts) != 1 || fake.hosts[0] != "localhost" {
		t.Errorf("pinged hosts = %v, want [localhost]", fake.hosts)
	}
}

func TestPingCommandPassesCountAndDeadline(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantTime  time.Duration
	}{
		{name: "defaults", args: nil, wantCount: 0, wantTime: ping.DefaultTimeout},
		{name: "count separate", args: []string{"--count", "1"}, wantCount: 1, wantTime: ping.DefaultTimeout},
		{name: "count joined", args: []string{"--count=10"}, wantCount: 10, wantTime: ping.DefaultTimeout},
		{name: "timeout separate", args: []string{"--timeout", "5s"}, wantCount: 0, wantTime: 5 * time.Second},
		{name: "timeout joined", args: []string{"--timeout=250ms"}, wantCount: 0, wantTime: 250 * time.Millisecond},
		{
			name:      "count and timeout together",
			args:      []string{"--count", "3", "--timeout", "1m"},
			wantCount: 3,
			wantTime:  time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, _, fake := newPingTestApp(nil)

			args := append([]string{"ping", "localhost"}, tt.args...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("Run(%v) returned error: %v", args, err)
			}

			if len(fake.options) != 1 {
				t.Fatalf("ping was called %d times, want 1", len(fake.options))
			}
			if fake.options[0].Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", fake.options[0].Count, tt.wantCount)
			}
			if len(fake.hasDeadline) != 1 || !fake.hasDeadline[0] {
				t.Fatalf("ping was called without a deadline: %v", fake.hasDeadline)
			}
			// The remaining budget is slightly below the configured timeout
			// because time passes between setting and observing it.
			if got := fake.deadlines[0]; got > tt.wantTime {
				t.Errorf("deadline budget = %v, want no more than %v", got, tt.wantTime)
			} else if got < tt.wantTime/2 {
				t.Errorf("deadline budget = %v, want close to %v", got, tt.wantTime)
			}
		})
	}
}

func TestParsePingArgsAcceptsHostAndFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantHost    string
		wantCount   int
		wantTimeout time.Duration
	}{
		{name: "host only", args: []string{"localhost"}, wantHost: "localhost", wantTimeout: ping.DefaultTimeout},
		{name: "ipv4 literal", args: []string{"127.0.0.1"}, wantHost: "127.0.0.1", wantTimeout: ping.DefaultTimeout},
		{name: "ipv6 literal", args: []string{"::1"}, wantHost: "::1", wantTimeout: ping.DefaultTimeout},
		{name: "flags before host", args: []string{"--count", "2", "example.com"}, wantHost: "example.com", wantCount: 2, wantTimeout: ping.DefaultTimeout},
		{name: "flags after host", args: []string{"example.com", "--count=7"}, wantHost: "example.com", wantCount: 7, wantTimeout: ping.DefaultTimeout},
		{name: "long timeout", args: []string{"example.com", "--timeout", "30s"}, wantHost: "example.com", wantTimeout: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, opts, timeout, err := parsePingArgs(tt.args)
			if err != nil {
				t.Fatalf("parsePingArgs(%v) returned error: %v", tt.args, err)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if opts.Count != tt.wantCount {
				t.Errorf("count = %d, want %d", opts.Count, tt.wantCount)
			}
			if timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", timeout, tt.wantTimeout)
			}
		})
	}
}

func TestParsePingArgsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing host", args: nil},
		{name: "only a flag", args: []string{"--count", "2"}},
		{name: "extra positional", args: []string{"a.com", "b.com"}},
		{name: "unknown flag", args: []string{"localhost", "--nope"}},
		{name: "count zero", args: []string{"localhost", "--count", "0"}},
		{name: "count negative", args: []string{"localhost", "--count", "-1"}},
		{name: "count not a number", args: []string{"localhost", "--count", "abc"}},
		{name: "count missing value", args: []string{"localhost", "--count"}},
		{name: "count float", args: []string{"localhost", "--count", "1.5"}},
		{name: "timeout not a duration", args: []string{"localhost", "--timeout", "abc"}},
		{name: "timeout zero", args: []string{"localhost", "--timeout", "0"}},
		{name: "timeout zero unit", args: []string{"localhost", "--timeout", "0s"}},
		{name: "timeout negative", args: []string{"localhost", "--timeout", "-1s"}},
		{name: "timeout empty", args: []string{"localhost", "--timeout="}},
		{name: "timeout missing value", args: []string{"localhost", "--timeout"}},
		{name: "timeout without unit", args: []string{"localhost", "--timeout", "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := parsePingArgs(tt.args)
			if err == nil {
				t.Fatalf("parsePingArgs(%v) succeeded, want an error", tt.args)
			}
			if !errors.Is(err, ErrUsage) {
				t.Errorf("error = %v, want it to wrap ErrUsage", err)
			}
		})
	}
}

func TestPingCommandRejectsBadInputWithExitCode2AndNoMeasurement(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing host", args: []string{"ping"}},
		{name: "two hosts", args: []string{"ping", "a.com", "b.com"}},
		{name: "unknown flag", args: []string{"ping", "localhost", "--nope"}},
		{name: "count zero", args: []string{"ping", "localhost", "--count", "0"}},
		{name: "count negative", args: []string{"ping", "localhost", "--count", "-2"}},
		{name: "count not a number", args: []string{"ping", "localhost", "--count", "abc"}},
		{name: "invalid timeout", args: []string{"ping", "localhost", "--timeout", "abc"}},
		{name: "zero timeout", args: []string{"ping", "localhost", "--timeout", "0"}},
		{name: "negative timeout", args: []string{"ping", "localhost", "--timeout", "-1s"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, stderr, fake := newPingTestApp(nil)

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
			// The whole point of validating first: no measurement is attempted.
			if len(fake.hosts) != 0 {
				t.Errorf("ping was called %d times despite invalid input", len(fake.hosts))
			}
		})
	}
}

func TestPingCommandRuntimeFailureIsExitCode1(t *testing.T) {
	sentinel := errors.New("open ICMP socket: permission denied")
	app, stdout, stderr, _ := newPingTestApp(func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		return ping.Report{}, sentinel
	})

	err := app.Run(context.Background(), []string{"ping", "localhost"})
	if err == nil {
		t.Fatal("Run returned nil error for a measurement failure")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the underlying error preserved", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
	// A runtime failure must not print usage, and must not print false results.
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("usage was printed for a runtime failure: %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("a failed measurement produced stdout output: %q", stdout.String())
	}
}

func TestPingCommandPacketLossIsNotAFailure(t *testing.T) {
	// Partial loss is a normal result: the command reports it and succeeds.
	app, stdout, _, _ := newPingTestApp(func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		report := okReport(host, 2)
		report.Result.Packets[1] = ping.Packet{Sequence: 2, Status: ping.StatusTimeout}
		report.Result.Received = 1
		return report, nil
	})

	if err := app.Run(context.Background(), []string{"ping", "localhost"}); err != nil {
		t.Fatalf("Run returned error for packet loss: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "TIMEOUT") {
		t.Errorf("output does not report the lost packet:\n%s", out)
	}
	if !strings.Contains(out, "Lost     : 1") {
		t.Errorf("summary does not report the loss:\n%s", out)
	}
}

func TestPingCommandAllPacketsLostIsStillASuccess(t *testing.T) {
	app, stdout, _, _ := newPingTestApp(func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		return ping.Report{Result: ping.Result{
			Host:    host,
			Address: "192.0.2.1",
			Packets: []ping.Packet{
				{Sequence: 1, Status: ping.StatusTimeout},
				{Sequence: 2, Status: ping.StatusTimeout},
			},
			Sent: 2,
		}}, nil
	})

	if err := app.Run(context.Background(), []string{"ping", "192.0.2.1", "--count", "2"}); err != nil {
		t.Fatalf("Run returned error for total packet loss: %v", err)
	}
	if !strings.Contains(stdout.String(), "Received : 0") {
		t.Errorf("output does not report zero received:\n%s", stdout.String())
	}
}

func TestPingCommandCancelledParentStillWins(t *testing.T) {
	// A cancelled parent must abort the run even though the command adds a
	// deadline of its own: the child derives from the parent, so Ctrl-C works.
	app, _, _, fake := newPingTestApp(func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		<-ctx.Done()
		return ping.Report{}, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx, []string{"ping", "localhost", "--timeout", "1m"})
	if err == nil {
		t.Fatal("Run returned nil error for a cancelled parent context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled preserved", err)
	}
	if len(fake.hosts) != 1 {
		t.Errorf("ping calls = %v, want exactly one attempt", fake.hosts)
	}
}

func TestPingCommandTimeoutIsClassifiedAsTimeout(t *testing.T) {
	// Exercise the real classification by delegating to ping.RunWith with a
	// transport that never answers, so the run's own deadline fires.
	app, _, stderr, _ := newPingTestApp(func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		return ping.RunWith(ctx, blockingEchoer{}, host, opts)
	})

	err := app.Run(context.Background(), []string{"ping", "localhost", "--timeout", "1ms"})
	if err == nil {
		t.Fatal("Run returned nil error for a timed-out run")
	}
	if !ping.IsTimeout(err) {
		t.Errorf("error = %v, want it recognisable as a timeout", err)
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error %v is not an *ExitError", err)
	}
	if exitErr.Code != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.Code)
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("usage was printed for a runtime failure: %q", stderr.String())
	}
}

// blockingEchoer never answers before the caller's deadline expires.
type blockingEchoer struct{}

func (blockingEchoer) Echo(ctx context.Context, ip net.IP, sequence int) (time.Duration, error) {
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestPingCommandReportsWriteFailure(t *testing.T) {
	app := New(failingWriter{}, io.Discard)
	app.ping = func(ctx context.Context, host string, opts ping.Options) (ping.Report, error) {
		return okReport(host, 1), nil
	}

	err := app.Run(context.Background(), []string{"ping", "localhost", "--count", "1"})
	if err == nil {
		t.Fatal("Run returned nil error for a failing writer")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("error = %q, want a write failure", err.Error())
	}
}

func TestPingCommandDefaultTransportIsTheRealOne(t *testing.T) {
	// A production App must reach the real transport, not a nil function.
	app, _, _, _ := newPingTestApp(nil)
	app.ping = nil

	if app.pinger() == nil {
		t.Fatal("pinger() returned nil for a production App")
	}
}
