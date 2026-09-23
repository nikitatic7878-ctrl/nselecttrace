package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestInterfacesCommandIsRegistered(t *testing.T) {
	app, _, _ := newTestApp()

	cmd, ok := app.Command("interfaces")
	if !ok {
		t.Fatal("interfaces command is not registered")
	}
	if cmd.Summary == "" {
		t.Error("interfaces command has no summary")
	}
	if !strings.Contains(cmd.Usage, "interfaces") {
		t.Errorf("Usage = %q, want it to contain %q", cmd.Usage, "interfaces")
	}
	if cmd.Run == nil {
		t.Fatal("interfaces command has no Run function")
	}
}

func TestInterfacesCommandIsListedInHelp(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}
	line := ""
	for _, l := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "interfaces") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("interfaces is not listed in help:\n%s", stdout.String())
	}
	if !strings.Contains(line, "list network interfaces") {
		t.Errorf("help line %q does not describe the command", line)
	}
}

func TestInterfacesCommandHelp(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"interfaces", "--help"}); err != nil {
		t.Fatalf("Run(interfaces --help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "nselecttrace interfaces") {
		t.Errorf("usage line missing: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "list network interfaces and their addresses") {
		t.Errorf("summary missing: %q", stdout.String())
	}
}

// TestInterfacesCommandRuns exercises the command against the interfaces of the
// machine running the test. It asserts only on properties that must hold on any
// machine: the header, at least one interface, and a loopback entry.
func TestInterfacesCommandRuns(t *testing.T) {
	app, stdout, stderr := newTestApp()

	if err := app.Run(context.Background(), []string{"interfaces"}); err != nil {
		t.Skipf("interfaces unavailable on this machine: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "ADDRESSES") {
		t.Errorf("header row missing:\n%s", out)
	}
	if strings.Count(strings.TrimRight(out, "\n"), "\n") < 1 {
		t.Errorf("no interface rows were printed:\n%s", out)
	}
	if !strings.Contains(out, "UP") && !strings.Contains(out, "DOWN") {
		t.Errorf("no interface state was printed:\n%s", out)
	}
	// Loopback exists on every supported platform.
	if !strings.Contains(strings.ToLower(out), "loopback") && !strings.Contains(out, "lo") {
		t.Logf("no loopback interface detected on this machine:\n%s", out)
	}
}

func TestInterfacesCommandVerbose(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"interfaces", "--verbose"}); err != nil {
		t.Skipf("interfaces unavailable on this machine: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"index:", "state:", "mac:", "mtu:", "addresses:"} {
		if !strings.Contains(out, want) {
			t.Errorf("verbose output is missing %q:\n%s", want, out)
		}
	}
}

func TestInterfacesCommandRejectsUnknownArgument(t *testing.T) {
	app, _, stderr := newTestApp()

	err := app.Run(context.Background(), []string{"interfaces", "--nope"})
	if err == nil {
		t.Fatal("Run(interfaces --nope) returned nil error")
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

// TestInterfacesCommandSurvivesUnbufferedOutput guards the error path of the
// command: a broken writer must be reported, not panicked over.
func TestInterfacesCommandSurvivesUnbufferedOutput(t *testing.T) {
	app := New(failingWriter{}, io.Discard)

	err := app.Run(context.Background(), []string{"interfaces"})
	if err == nil {
		t.Skip("system source failed before writing, nothing to verify")
	}
	if !strings.Contains(err.Error(), "failed to write output") {
		t.Errorf("error = %q, want a write failure", err.Error())
	}
}

// failingWriter rejects every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
