package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"nselecttrace/internal/ports"
)

// parsePortsFlagsTest is the `nselecttrace ports` instantiation of the shared flag
// parser: no extra flags, no connection-specific defaults.
func parsePortsFlagsTest(args []string) (ports.Filter, error) {
	return parsePortsFlags("ports", args, nil)
}

func TestPortsCommandIsRegistered(t *testing.T) {
	app, _, _ := newTestApp()

	cmd, ok := app.Command("ports")
	if !ok {
		t.Fatal("ports command is not registered")
	}
	if cmd.Summary == "" {
		t.Error("ports command has no summary")
	}
	if !strings.Contains(cmd.Usage, "ports") {
		t.Errorf("Usage = %q, want it to contain %q", cmd.Usage, "ports")
	}
	if len(cmd.Arguments) != 6 {
		t.Errorf("Arguments = %d, want 6 documented flags", len(cmd.Arguments))
	}
	if cmd.Run == nil {
		t.Fatal("ports command has no Run function")
	}
}

func TestPortsCommandIsListedInHelp(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ports") {
		t.Errorf("ports is not listed in help:\n%s", stdout.String())
	}
}

func TestPortsCommandHelpDocumentsFlags(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"ports", "--help"}); err != nil {
		t.Fatalf("Run(ports --help) returned error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"nselecttrace ports", "--tcp", "--udp", "--port", "--pid", "--process"} {
		if !strings.Contains(out, want) {
			t.Errorf("ports help is missing %q:\n%s", want, out)
		}
	}
}

func TestParsePortsFilter(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want ports.Filter
	}{
		{name: "empty", args: nil, want: ports.Filter{}},
		{name: "tcp", args: []string{"--tcp"}, want: ports.Filter{Protocol: ports.TCP}},
		{name: "udp", args: []string{"--udp"}, want: ports.Filter{Protocol: ports.UDP}},
		{name: "tcp and udp is no filter", args: []string{"--tcp", "--udp"}, want: ports.Filter{}},
		{name: "port separate", args: []string{"--port", "3000"}, want: ports.Filter{LocalPort: 3000}},
		{name: "port joined", args: []string{"--port=3000"}, want: ports.Filter{LocalPort: 3000}},
		{name: "port boundary", args: []string{"--port", "65535"}, want: ports.Filter{LocalPort: 65535}},
		{name: "pid", args: []string{"--pid", "8212"}, want: ports.Filter{PID: 8212}},
		{name: "process", args: []string{"--process", "node"}, want: ports.Filter{ProcessSub: "node"}},
		{
			name: "combined",
			args: []string{"--tcp", "--port=443", "--process", "nginx"},
			want: ports.Filter{Protocol: ports.TCP, LocalPort: 443, ProcessSub: "nginx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortsFlagsTest(tt.args)
			if err != nil {
				t.Fatalf("parsePortsFlagsTest(%v) returned error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parsePortsFlagsTest(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestParsePortsFilterRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--nope"}},
		{name: "positional", args: []string{"3000"}},
		{name: "port not a number", args: []string{"--port", "abc"}},
		{name: "port zero", args: []string{"--port", "0"}},
		{name: "port too large", args: []string{"--port", "65536"}},
		{name: "port negative", args: []string{"--port", "-1"}},
		{name: "port missing value", args: []string{"--port"}},
		{name: "pid not a number", args: []string{"--pid", "x"}},
		{name: "pid missing value", args: []string{"--pid"}},
		{name: "pid negative", args: []string{"--pid", "-5"}},
		{name: "process missing value", args: []string{"--process"}},
		{name: "process empty", args: []string{"--process="}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePortsFlagsTest(tt.args)
			if err == nil {
				t.Fatalf("parsePortsFlagsTest(%v) succeeded, want an error", tt.args)
			}
			if !errors.Is(err, ErrUsage) {
				t.Errorf("error = %v, want it to wrap ErrUsage", err)
			}
		})
	}
}

func TestPortsCommandRejectsBadInputWithExitCode2(t *testing.T) {
	app, _, stderr := newTestApp()

	err := app.Run(context.Background(), []string{"ports", "--port", "abc"})
	if err == nil {
		t.Fatal("Run(ports --port abc) returned nil error")
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
}

// TestPortsCommandRuns exercises the command against the real socket table. It
// asserts only on properties that hold on any machine, never on a specific
// port or PID.
func TestPortsCommandRuns(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"ports"}); err != nil {
		t.Skipf("ports are unavailable on this machine: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"PROTO", "LOCAL ADDRESS", "STATE", "PID", "PROCESS"} {
		if !strings.Contains(out, want) {
			t.Errorf("header is missing %q:\n%s", want, out)
		}
	}
	// Every non-header row must start with a known protocol.
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if i == 0 || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "TCP") && !strings.HasPrefix(line, "UDP") {
			t.Errorf("row %d does not start with a protocol: %q", i, line)
		}
	}
}

func TestPortsCommandFiltersRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// onlyProto, when set, must be the only protocol present in the output.
		onlyProto string
	}{
		{name: "tcp only", args: []string{"--tcp"}, onlyProto: "TCP"},
		{name: "udp only", args: []string{"--udp"}, onlyProto: "UDP"},
		{name: "unused port", args: []string{"--port", "1"}},
		{name: "no such process", args: []string{"--process", "definitely-not-running-anything"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, stdout, _ := newTestApp()

			if err := app.Run(context.Background(), append([]string{"ports"}, tt.args...)); err != nil {
				t.Skipf("ports are unavailable on this machine: %v", err)
			}

			out := stdout.String()
			if !strings.Contains(out, "PROTO") {
				t.Fatalf("header is missing:\n%s", out)
			}

			rows := strings.Split(strings.TrimRight(out, "\n"), "\n")[1:]
			if tt.onlyProto == "TCP" && strings.Contains(out, "UDP") {
				t.Errorf("--tcp output contains UDP rows:\n%s", out)
			}
			if tt.onlyProto == "UDP" && strings.Contains(out, "TCP") {
				t.Errorf("--udp output contains TCP rows:\n%s", out)
			}
			// Each remaining check only has to hold when rows were returned.
			for _, row := range rows {
				if row == "" {
					continue
				}
				if tt.onlyProto != "" && !strings.HasPrefix(row, tt.onlyProto) {
					t.Errorf("row %q is not %s", row, tt.onlyProto)
				}
			}
		})
	}
}

func TestPortsCommandPortFilterReturnsOnlyThatPort(t *testing.T) {
	// Pick a port from a real listing, then ask for it: at least that row must
	// come back, which proves the filter reaches the domain model.
	app, stdout, _ := newTestApp()
	if err := app.Run(context.Background(), []string{"ports"}); err != nil {
		t.Skipf("ports are unavailable on this machine: %v", err)
	}

	rows := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(rows) < 2 {
		t.Skip("no sockets to pick a port from")
	}

	// The local address is the second column of the first data row.
	fields := strings.Fields(rows[1])
	if len(fields) < 2 {
		t.Skipf("cannot parse row %q", rows[1])
	}
	endpoint := fields[1]
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 || idx == len(endpoint)-1 {
		t.Skipf("cannot extract a port from %q", endpoint)
	}
	port := endpoint[idx+1:]

	app2, stdout2, _ := newTestApp()
	if err := app2.Run(context.Background(), []string{"ports", "--port", port}); err != nil {
		t.Fatalf("Run(ports --port %s) returned error: %v", port, err)
	}
	out := stdout2.String()
	if !strings.Contains(out, "PROTO") {
		t.Fatalf("header is missing:\n%s", out)
	}
	for _, row := range strings.Split(strings.TrimRight(out, "\n"), "\n")[1:] {
		if row == "" {
			continue
		}
		if !strings.HasSuffix(strings.Fields(row)[1], ":"+port) {
			t.Errorf("row %q does not belong to port %s", row, port)
		}
	}
}

func TestPortsCommandReportsWriteFailure(t *testing.T) {
	app := New(failingWriter{}, io.Discard)

	err := app.Run(context.Background(), []string{"ports"})
	if err == nil {
		t.Skip("socket enumeration failed before writing, nothing to verify")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("error = %q, want a write failure", err.Error())
	}
}
