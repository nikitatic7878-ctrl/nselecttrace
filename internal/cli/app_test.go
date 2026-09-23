package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func newTestApp() (*App, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return New(stdout, stderr), stdout, stderr
}

func TestRunWithoutArgumentsPrintsBanner(t *testing.T) {
	app, stdout, stderr := newTestApp()

	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run(nil) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), programName) {
		t.Errorf("banner does not mention %q: %q", programName, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("banner is missing the usage block: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}
}

func TestRunAbout(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"about"}); err != nil {
		t.Fatalf("Run(about) returned error: %v", err)
	}
	for _, want := range []string{programName, "Network diagnostic utility", "host:", "runtime:", "version:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("about output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunHelpVariants(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			app, stdout, _ := newTestApp()

			if err := app.Run(context.Background(), []string{arg}); err != nil {
				t.Fatalf("Run(%q) returned error: %v", arg, err)
			}
			for _, want := range []string{"Usage:", "Available commands:", "about", "version"} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("help output missing %q:\n%s", want, stdout.String())
				}
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	for _, arg := range []string{"version", "-v", "--version"} {
		t.Run(arg, func(t *testing.T) {
			app, stdout, _ := newTestApp()

			if err := app.Run(context.Background(), []string{arg}); err != nil {
				t.Fatalf("Run(%q) returned error: %v", arg, err)
			}
			if !strings.Contains(stdout.String(), programName) {
				t.Errorf("version output does not mention %q: %q", programName, stdout.String())
			}
		})
	}
}

func TestRunUnknownCommand(t *testing.T) {
	app, _, stderr := newTestApp()

	err := app.Run(context.Background(), []string{"does-not-exist"})
	if err == nil {
		t.Fatal("Run with an unknown command returned nil error")
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

func TestCommandHelpShortCircuits(t *testing.T) {
	app, stdout, _ := newTestApp()

	if err := app.Run(context.Background(), []string{"about", "--help"}); err != nil {
		t.Fatalf("Run(about --help) returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "nselecttrace about") {
		t.Errorf("command usage not printed: %q", stdout.String())
	}
}

func TestAddIgnoresDuplicateNames(t *testing.T) {
	app, _, _ := newTestApp()
	before := len(app.Commands())

	app.Add(Command{Name: "version", Summary: "duplicate", Usage: "version"})
	app.Add(Command{Name: "extra", Summary: "extra", Usage: "extra"})

	if got := len(app.Commands()); got != before+1 {
		t.Errorf("command count = %d, want %d", got, before+1)
	}
	if _, ok := app.Command("extra"); !ok {
		t.Error("extra command was not registered")
	}
}
