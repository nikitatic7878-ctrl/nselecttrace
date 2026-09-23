package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nselecttrace/internal/dns"
)

// resolve is the seam the dns command uses to reach the resolver.
//
// It is a field on App rather than a direct call to dns.Resolve so that CLI
// tests can supply a fake and assert on success, failure and timeout handling
// without a network. Production keeps dns.Resolve, so behaviour is unchanged.
type resolveFunc func(ctx context.Context, host string) (dns.Result, error)

// dnsCommand resolves a host name to IP addresses.
//
// All resolution and rendering lives in internal/dns; this file parses the
// command line, turns --timeout into a context deadline and maps failures onto
// the shared exit-code model. It never touches net.Resolver itself.
func (a *App) dnsCommand() Command {
	return Command{
		Name:    "dns",
		Summary: "resolve a host name to IP addresses",
		Usage:   "dns <host> [--timeout <duration>]",
		Arguments: []Argument{
			{Name: "<host>", Help: "host name to resolve, for example example.com"},
			{Name: "--timeout <duration>", Help: "resolution timeout, for example 5s or 500ms (default " + defaultTimeoutText() + ")"},
		},
		Run: a.runDNS,
	}
}

func (a *App) runDNS(ctx context.Context, args []string) error {
	host, timeout, err := parseDNSArgs(args)
	if err != nil {
		return err
	}

	// The deadline is derived from the incoming context, so a cancelled parent
	// (Ctrl-C via signal.NotifyContext) still aborts immediately and wins over
	// the timeout. No context is created from scratch here.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := a.resolver()(ctx, host)
	if err != nil {
		// main.go adds the "nselecttrace: " prefix, so it must not be repeated
		// here or the message would read "nselecttrace: nselecttrace: ...".
		return WithCode(1, err)
	}

	if err := dns.WriteTable(a.stdout, result); err != nil {
		return WithCode(1, fmt.Errorf("failed to write output: %w", err))
	}
	return nil
}

// resolver returns the resolver function to use, defaulting to dns.Resolve.
func (a *App) resolver() resolveFunc {
	if a.resolve != nil {
		return a.resolve
	}
	return dns.Resolve
}

// parseDNSArgs validates the command line and returns the host and the timeout.
//
// Every failure here is a user-input error, so the App turns it into exit code 2
// with usage on stderr, and no resolution is attempted.
func parseDNSArgs(args []string) (string, time.Duration, error) {
	var (
		host    string
		timeout = dns.DefaultTimeout
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
				return "", UsageError("dns: %s requires a value", name)
			}
			return args[i], nil
		}

		switch name {
		case "--timeout":
			raw, err := takeValue()
			if err != nil {
				return "", 0, err
			}
			parsed, err := parseTimeout("dns", raw)
			if err != nil {
				return "", 0, err
			}
			timeout = parsed
		default:
			if strings.HasPrefix(arg, "-") {
				return "", 0, UsageError("dns: unknown flag %q", arg)
			}
			if host != "" {
				return "", 0, UsageError("dns: expected exactly one host, got %q in addition to %q", arg, host)
			}
			host = arg
		}
	}

	if host == "" {
		return "", 0, UsageError("dns: missing host name")
	}
	return host, timeout, nil
}

// parseTimeout validates a --timeout value for the named command.
//
// A non-positive duration is rejected: zero would mean "already expired" and a
// negative value is meaningless, both of which would look like a network fault
// instead of a typo.
func parseTimeout(command, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, UsageError("%s: invalid timeout %q (want a duration such as 5s or 500ms)", command, raw)
	}
	if d <= 0 {
		return 0, UsageError("%s: invalid timeout %q (want a positive duration)", command, raw)
	}
	return d, nil
}

// defaultTimeoutText renders dns.DefaultTimeout for the help output.
func defaultTimeoutText() string {
	return dns.DefaultTimeout.String()
}
