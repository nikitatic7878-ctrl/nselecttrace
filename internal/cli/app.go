// Package cli implements the nselecttrace command-line front end.
//
// The package only knows how to parse arguments, route them to a registered
// command and render help or errors. All data collection lives in the
// internal/* packages so that the CLI and a future TUI can share it.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"nselecttrace/internal/buildinfo"
)

// ExitError carries a specific process exit code while keeping the error
// message visible to the user.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }

// Unwrap reports the underlying error so errors.Is/errors.As keep working.
func (e *ExitError) Unwrap() error { return e.Err }

// WithCode wraps err so that the process exits with code.
func WithCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

// ErrUsage indicates that the command line itself was invalid.
var ErrUsage = errors.New("invalid usage")

// UsageError reports an invalid command line, for example a missing operand.
func UsageError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUsage, fmt.Sprintf(format, args...))
}

// App is a command line application with a fixed set of commands.
type App struct {
	stdout io.Writer
	stderr io.Writer
	order  []Command
	byName map[string]Command

	// resolve performs DNS lookups for the dns command. It is nil in
	// production, where dns.Resolve is used; tests set it to a fake so that no
	// test depends on the network. This is the same seam idea as the
	// systemSource in internal/ports, kept on the App because that is what a
	// command has access to.
	resolve resolveFunc

	// ping performs the measurements for the ping command. It is nil in
	// production, where ping.Run is used, and set to a fake in tests for the
	// same reason as resolve.
	ping pingFunc
}

// New returns an App with the built-in command set registered.
func New(stdout, stderr io.Writer) *App {
	app := &App{
		stdout: stdout,
		stderr: stderr,
		byName: map[string]Command{},
	}
	app.init()
	return app
}

// Run routes args to a command and returns its error, if any.
func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.WriteUsage(a.stdout)
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.WriteUsage(a.stdout)
		return nil
	case "-v", "--version", "version":
		return a.runVersion(ctx, args[1:])
	}

	name := args[0]
	cmd, ok := a.byName[name]
	if !ok {
		a.WriteUsage(a.stderr)
		return WithCode(2, UsageError("unknown command %q", name))
	}

	rest := args[1:]
	for _, arg := range rest {
		if arg == "-h" || arg == "--help" {
			a.writeCommandUsage(a.stdout, cmd)
			return nil
		}
	}

	if err := cmd.Run(ctx, rest); err != nil {
		if errors.Is(err, ErrUsage) {
			a.writeCommandUsage(a.stderr, cmd)
			return WithCode(2, err)
		}
		return err
	}
	return nil
}

func (a *App) runVersion(_ context.Context, args []string) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			a.WriteUsage(a.stdout)
			return nil
		}
	}
	fmt.Fprintf(a.stdout, "%s %s\n", programName, buildinfo.Version)
	return nil
}

// Add registers cmd, keeping a stable order for the help output.
func (a *App) Add(cmds ...Command) {
	for _, cmd := range cmds {
		if _, exists := a.byName[cmd.Name]; exists {
			continue
		}
		a.byName[cmd.Name] = cmd
		a.order = append(a.order, cmd)
	}
}

// Commands returns the registered commands in registration order.
func (a *App) Commands() []Command {
	return append([]Command(nil), a.order...)
}

// Command looks up a registered command by name.
func (a *App) Command(name string) (Command, bool) {
	cmd, ok := a.byName[name]
	return cmd, ok
}

// Usage returns the text a.WriteUsage writes.
func (a *App) Usage() string {
	var b strings.Builder
	fprintUsage(&b, a.order)
	return b.String()
}

// WriteUsage writes the application help to w.
func (a *App) WriteUsage(w io.Writer) { fmt.Fprint(w, a.Usage()) }

func (a *App) writeCommandUsage(w io.Writer, cmd Command) {
	var b strings.Builder
	fprintCommandUsage(&b, cmd)
	fmt.Fprint(w, b.String())
}

func fprintUsage(w io.Writer, cmds []Command) {
	fmt.Fprintf(w, "%s %s - A lightweight terminal network diagnostic and inspection tool.\n\n", programName, buildinfo.Version)
	fmt.Fprintf(w, "Usage:\n  %s <command> [arguments]\n\n", programName)
	fmt.Fprintf(w, "Available commands:\n%s\n", commandList(cmds))
	fmt.Fprintf(w, "Flags:\n  -h, --help      show help\n  -v, --version   show version\n")
	fmt.Fprintf(w, "\nRun '%s <command> --help' for details about a command.\n", programName)
}

func fprintCommandUsage(w io.Writer, cmd Command) {
	fmt.Fprintf(w, "Usage:\n  %s %s\n\n", programName, cmd.Usage)
	if cmd.Summary != "" {
		fmt.Fprintf(w, "%s\n\n", cmd.Summary)
	}
	if len(cmd.Arguments) == 0 {
		return
	}
	fmt.Fprintln(w, "Arguments:")
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, arg := range cmd.Arguments {
		fmt.Fprintf(tw, "  %s\t%s\n", arg.Name, arg.Help)
	}
	_ = tw.Flush()
}

func commandList(cmds []Command) string {
	sorted := append([]Command(nil), cmds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	for _, cmd := range sorted {
		fmt.Fprintf(tw, "  %s\t%s\n", cmd.Name, cmd.Summary)
	}
	_ = tw.Flush()
	return strings.TrimRight(b.String(), "\n")
}

// hostName returns the local host name, or "unknown" when it cannot be read.
func hostName() string {
	name, err := osHostname()
	if err != nil {
		return "unknown"
	}
	return name
}

// runtimeInfo returns a human readable description of the running platform.
func runtimeInfo() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
