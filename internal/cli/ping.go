package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nselecttrace/internal/ping"
)

// pingFunc is the seam the ping command uses to reach the network.
//
// It is a field on App rather than a direct call to ping.Run so that CLI tests
// can supply a fake and assert on success, packet loss, failure and timeout
// handling without ICMP, DNS or elevated privileges. Production keeps ping.Run,
// so behaviour is unchanged. This mirrors the resolver seam used by dns.
type pingFunc func(ctx context.Context, host string, opts ping.Options) (ping.Report, error)

// pingCommand measures ICMP echo reachability of a host.
//
// All measurement and rendering lives in internal/ping; this file parses the
// command line, turns --timeout into a context deadline and maps failures onto
// the shared exit-code model.
func (a *App) pingCommand() Command {
	return Command{
		Name:    "ping",
		Summary: "measure ICMP echo reachability of a host",
		Usage:   "ping <host> [--count <n>] [--timeout <duration>]",
		Arguments: []Argument{
			{Name: "<host>", Help: "host name or address to ping, for example example.com"},
			{Name: "--count <n>", Help: "number of echo requests to send (default " + strconv.Itoa(ping.DefaultCount) + ")"},
			{Name: "--timeout <duration>", Help: "overall timeout, for example 5s or 500ms (default " + defaultTimeoutText() + ")"},
		},
		Run: a.runPing,
	}
}

func (a *App) runPing(ctx context.Context, args []string) error {
	host, opts, timeout, err := parsePingArgs(args)
	if err != nil {
		return err
	}

	// The deadline is derived from the incoming context, so a cancelled parent
	// (Ctrl-C via signal.NotifyContext) still aborts the run immediately and
	// wins over the timeout. No context is created from scratch here.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	report, err := a.pinger()(ctx, host, opts)
	if err != nil {
		// main.go adds the "nselecttrace: " prefix, so it must not be repeated
		// here. A cancelled run is reported as a failure, not as packet loss.
		return WithCode(1, err)
	}

	if err := ping.WriteTable(a.stdout, report); err != nil {
		return WithCode(1, fmt.Errorf("failed to write output: %w", err))
	}
	return nil
}

// pinger returns the ping function to use, defaulting to ping.Run.
func (a *App) pinger() pingFunc {
	if a.ping != nil {
		return a.ping
	}
	return ping.Run
}

// parsePingArgs validates the command line and returns the host, the run options
// and the timeout.
//
// Every failure here is a user-input error, so the App turns it into exit code 2
// with usage on stderr, and no measurement is attempted.
func parsePingArgs(args []string) (string, ping.Options, time.Duration, error) {
	var (
		host    string
		opts    ping.Options
		timeout = ping.DefaultTimeout
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")

		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", UsageError("ping: %s requires a value", name)
			}
			return args[i], nil
		}

		switch name {
		case "--count":
			raw, err := takeValue()
			if err != nil {
				return "", opts, 0, err
			}
			count, err := parseCount(raw)
			if err != nil {
				return "", opts, 0, err
			}
			opts.Count = count
		case "--timeout":
			raw, err := takeValue()
			if err != nil {
				return "", opts, 0, err
			}
			parsed, err := parseTimeout("ping", raw)
			if err != nil {
				return "", opts, 0, err
			}
			timeout = parsed
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, 0, UsageError("ping: unknown flag %q", arg)
			}
			if host != "" {
				return "", opts, 0, UsageError("ping: expected exactly one host, got %q in addition to %q", arg, host)
			}
			host = arg
		}
	}

	if host == "" {
		return "", opts, 0, UsageError("ping: missing host")
	}
	return host, opts, timeout, nil
}

// parseCount validates a --count value.
//
// A count of zero or less is rejected: this version sends a bounded number of
// requests, and there is no continuous mode to fall back to.
func parseCount(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, UsageError("ping: invalid count %q (want a positive number)", raw)
	}
	return n, nil
}
