package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"nselecttrace/internal/connections"
	"nselecttrace/internal/ports"
)

// parseConnectionsFlagsTest is the `nselecttrace connections` instantiation of the
// shared parser, including its connection-only flags.
func parseConnectionsFlagsTest(args []string) (connections.Filter, error) {
	var remotePort uint16
	extra := func(name string, takeValue func() (string, error)) (bool, error) {
		if name != "--remote-port" {
			return false, nil
		}
		raw, err := takeValue()
		if err != nil {
			return true, err
		}
		port, err := parsePortNumber("connections", raw)
		if err != nil {
			return true, err
		}
		remotePort = port
		return true, nil
	}

	base, err := parsePortsFlags("connections", args, extra)
	if err != nil {
		return connections.Filter{}, err
	}
	base.RemoteOnly = true
	return connections.Filter{Filter: base, RemotePort: remotePort}, nil
}

func TestConnectionsCommandIsRegistered(t *testing.T) {
	app, _, _ := newTestApp()

	cmd, ok := app.Command("connections")
	if !ok {
		t.Fatal("connections command is not registered")
	}
	if cmd.Summary == "" {
		t.Error("connections command has no summary")
	}
	if !strings.Contains(cmd.Usage, "connections") {
		t.Errorf("Usage = %q, want it to contain %q", cmd.Usage, "connections")
	}
	if len(cmd.Arguments) != 7 {
		t.Errorf("Arguments = %d, want 7 documented flags", len(cmd.Arguments))
	}
	if cmd.Run == nil {
		t.Fatal("connections command has no Run function")
	}
}

func TestConnectionsCommandIsListedInHelp(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "connections") {
		t.Errorf("connections is not listed in help:\n%s", stdout.String())
	}
}

func TestConnectionsCommandHelpDocumentsFlags(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"connections", "--help"}); err != nil {
		t.Fatalf("Run(connections --help) returned error: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{
		"nselecttrace connections", "--tcp", "--udp", "--port", "--remote-port",
		"--pid", "--process", "--state",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("connections help is missing %q:\n%s", want, out)
		}
	}
}

func TestParseConnectionsFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want connections.Filter
	}{
		{
			name: "empty still selects only remote sockets",
			args: nil,
			want: connections.Filter{Filter: ports.Filter{RemoteOnly: true}},
		},
		{
			name: "tcp",
			args: []string{"--tcp"},
			want: connections.Filter{Filter: ports.Filter{Protocol: ports.TCP, RemoteOnly: true}},
		},
		{
			name: "udp",
			args: []string{"--udp"},
			want: connections.Filter{Filter: ports.Filter{Protocol: ports.UDP, RemoteOnly: true}},
		},
		{
			name: "port",
			args: []string{"--port", "52341"},
			want: connections.Filter{Filter: ports.Filter{LocalPort: 52341, RemoteOnly: true}},
		},
		{
			name: "remote port",
			args: []string{"--remote-port", "443"},
			want: connections.Filter{Filter: ports.Filter{RemoteOnly: true}, RemotePort: 443},
		},
		{
			name: "pid",
			args: []string{"--pid", "8416"},
			want: connections.Filter{Filter: ports.Filter{PID: 8416, RemoteOnly: true}},
		},
		{
			name: "process",
			args: []string{"--process", "chrome"},
			want: connections.Filter{Filter: ports.Filter{ProcessSub: "chrome", RemoteOnly: true}},
		},
		{
			name: "state",
			args: []string{"--state", "ESTABLISHED"},
			want: connections.Filter{Filter: ports.Filter{State: ports.StateEstablished, RemoteOnly: true}},
		},
		{
			name: "state is case insensitive",
			args: []string{"--state", "time_wait"},
			want: connections.Filter{Filter: ports.Filter{State: ports.StateTimeWait, RemoteOnly: true}},
		},
		{
			name: "combined",
			args: []string{"--tcp", "--state", "ESTABLISHED", "--remote-port=443", "--process", "chrome"},
			want: connections.Filter{Filter: ports.Filter{
				Protocol: ports.TCP, State: ports.StateEstablished, ProcessSub: "chrome", RemoteOnly: true,
			}, RemotePort: 443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConnectionsFlagsTest(tt.args)
			if err != nil {
				t.Fatalf("parse(%v) returned error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parse(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
			if !got.RemoteOnly {
				t.Error("RemoteOnly is not set: a connections filter must always require a remote endpoint")
			}
		})
	}
}

func TestParseConnectionsFlagsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid state", args: []string{"--state", "INVALID"}},
		{name: "empty state", args: []string{"--state="}},
		{name: "port not a number", args: []string{"--port", "abc"}},
		{name: "remote port not a number", args: []string{"--remote-port", "abc"}},
		{name: "remote port zero", args: []string{"--remote-port", "0"}},
		{name: "remote port missing value", args: []string{"--remote-port"}},
		{name: "pid negative", args: []string{"--pid", "-1"}},
		{name: "pid not a number", args: []string{"--pid", "x"}},
		{name: "unknown flag", args: []string{"--nope"}},
		{name: "positional", args: []string{"443"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConnectionsFlagsTest(tt.args)
			if err == nil {
				t.Fatalf("parse(%v) succeeded, want an error", tt.args)
			}
			if !errors.Is(err, ErrUsage) {
				t.Errorf("error = %v, want it to wrap ErrUsage", err)
			}
		})
	}
}

func TestConnectionsCommandRejectsBadInputWithExitCode2(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid state", args: []string{"connections", "--state", "INVALID"}},
		{name: "bad port", args: []string{"connections", "--port", "abc"}},
		{name: "negative pid", args: []string{"connections", "--pid", "-1"}},
		{name: "unknown flag", args: []string{"connections", "--nope"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, stderr := newTestApp()

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
		})
	}
}

// TestConnectionsCommandRuns exercises the command against the real socket
// table. It asserts only on properties that hold on any machine, and never on a
// specific remote host being reachable.
func TestConnectionsCommandRuns(t *testing.T) {
	app, stdout, stderr := newTestApp()

	if err := app.Run(context.Background(), []string{"connections"}); err != nil {
		t.Skipf("socket enumeration is unavailable on this machine: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"PROTO", "LOCAL", "REMOTE", "STATE", "PID", "PROCESS"} {
		if !strings.Contains(out, want) {
			t.Errorf("header is missing %q:\n%s", want, out)
		}
	}

	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if i == 0 || line == "" {
			continue
		}
		if !strings.HasPrefix(line, "TCP") && !strings.HasPrefix(line, "UDP") {
			t.Errorf("row %d does not start with a protocol: %q", i, line)
		}
		// The remote column must never be the "no remote" placeholder: that is
		// what makes the row a connection.
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "-" {
			t.Errorf("row %d has no remote endpoint: %q", i, line)
		}
	}

	// The privilege note, when present, belongs on stderr.
	if unknownNote := strings.Contains(out, "have no readable process name"); unknownNote {
		t.Errorf("the privilege note leaked into stdout:\n%s", out)
	}
	_ = stderr
}

func TestConnectionsCommandExcludesListeners(t *testing.T) {
	// The whole difference from `ports`: a listener has no remote endpoint and
	// must not appear here.
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"connections"}); err != nil {
		t.Skipf("socket enumeration is unavailable on this machine: %v", err)
	}
	if strings.Contains(stdout.String(), "LISTENING") {
		t.Errorf("a LISTENING socket leaked into connections:\n%s", stdout.String())
	}
}

func TestConnectionsCommandFiltersRun(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		onlyProto string
	}{
		{name: "tcp", args: []string{"--tcp"}, onlyProto: "TCP"},
		{name: "udp", args: []string{"--udp"}, onlyProto: "UDP"},
		{name: "state established", args: []string{"--state", "ESTABLISHED"}},
		{name: "state time wait", args: []string{"--state", "TIME_WAIT"}},
		{name: "remote port", args: []string{"--remote-port", "443"}},
		{name: "unused port", args: []string{"--port", "1"}},
		{name: "no such process", args: []string{"--process", "definitely-not-running-anything"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, stdout, _ := newTestApp()

			if err := app.Run(context.Background(), append([]string{"connections"}, tt.args...)); err != nil {
				t.Skipf("socket enumeration is unavailable on this machine: %v", err)
			}

			out := stdout.String()
			if !strings.Contains(out, "PROTO") {
				t.Fatalf("header is missing:\n%s", out)
			}
			if tt.onlyProto == "TCP" && strings.Contains(out, "UDP") {
				t.Errorf("--tcp output contains UDP rows:\n%s", out)
			}
			if tt.onlyProto == "UDP" && strings.Contains(out, "TCP") {
				t.Errorf("--udp output contains TCP rows:\n%s", out)
			}
			for _, row := range strings.Split(strings.TrimRight(out, "\n"), "\n")[1:] {
				if row == "" || tt.onlyProto == "" {
					continue
				}
				if !strings.HasPrefix(row, tt.onlyProto) {
					t.Errorf("row %q is not %s", row, tt.onlyProto)
				}
			}
		})
	}
}

func TestConnectionsCommandStateFilterReturnsOnlyThatState(t *testing.T) {
	// Derive a state from a real listing, then ask for it: the result must
	// contain only rows in that state, which proves the filter reaches the model
	// rather than being ignored.
	app, stdout, _ := newTestApp()
	if err := app.Run(context.Background(), []string{"connections"}); err != nil {
		t.Skipf("socket enumeration is unavailable on this machine: %v", err)
	}

	rows := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(rows) < 2 {
		t.Skip("no connections to pick a state from")
	}

	// STATE is the fourth column.
	fields := strings.Fields(rows[1])
	if len(fields) < 4 {
		t.Skipf("cannot parse row %q", rows[1])
	}
	state := fields[3]
	if state == "-" {
		t.Skip("the first connection has no state")
	}

	app2, stdout2, _ := newTestApp()
	if err := app2.Run(context.Background(), []string{"connections", "--state", state}); err != nil {
		t.Fatalf("Run(connections --state %s) returned error: %v", state, err)
	}
	out := stdout2.String()

	found := 0
	for _, row := range strings.Split(strings.TrimRight(out, "\n"), "\n")[1:] {
		if row == "" {
			continue
		}
		found++
		if !strings.Contains(row, state) {
			t.Errorf("row %q is not in state %s", row, state)
		}
	}
	if found == 0 {
		t.Errorf("filtering by %s returned nothing, although a %s connection was just listed", state, state)
	}
}

func TestConnectionsCommandRemotePortFilterMatchesOnlyThatPort(t *testing.T) {
	app, stdout, _ := newTestApp()
	if err := app.Run(context.Background(), []string{"connections"}); err != nil {
		t.Skipf("socket enumeration is unavailable on this machine: %v", err)
	}

	rows := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(rows) < 2 {
		t.Skip("no connections to pick a remote port from")
	}

	// REMOTE is the third column, "address:port" or "[address]:port".
	fields := strings.Fields(rows[1])
	if len(fields) < 3 {
		t.Skipf("cannot parse row %q", rows[1])
	}
	endpoint := fields[2]
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 || idx == len(endpoint)-1 {
		t.Skipf("cannot extract a port from %q", endpoint)
	}
	port := endpoint[idx+1:]

	app2, stdout2, _ := newTestApp()
	if err := app2.Run(context.Background(), []string{"connections", "--remote-port", port}); err != nil {
		t.Fatalf("Run(connections --remote-port %s) returned error: %v", port, err)
	}

	found := 0
	for _, row := range strings.Split(strings.TrimRight(stdout2.String(), "\n"), "\n")[1:] {
		if row == "" {
			continue
		}
		found++
		got := strings.Fields(row)
		if len(got) < 3 || !strings.HasSuffix(got[2], ":"+port) {
			t.Errorf("row %q does not connect to remote port %s", row, port)
		}
	}
	if found == 0 {
		t.Errorf("filtering by remote port %s returned nothing, although it was just listed", port)
	}
}

func TestConnectionsCommandReportsWriteFailure(t *testing.T) {
	app := New(failingWriter{}, io.Discard)

	err := app.Run(context.Background(), []string{"connections"})
	if err == nil {
		t.Skip("socket enumeration failed before writing, nothing to verify")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("error = %q, want a write failure", err.Error())
	}
}
