package cli

import "context"

// programName is the canonical name used in all usage and error messages.
const programName = "nselecttrace"

// Command is a single nselecttrace subcommand.
//
// Implementations collect and render data; the App only handles argument
// routing, help output and exit codes. Grouping a command together with its
// metadata keeps new commands additive: adding one never requires editing
// main.go or the App itself.
type Command struct {
	// Name is the token that selects the command on the command line.
	Name string
	// Summary is the one-line description shown in the command list.
	Summary string
	// Usage is the argument string shown after "nselecttrace", e.g. "ping <host>".
	Usage string
	// Arguments is optional per-argument documentation for the command help.
	Arguments []Argument
	// Run performs the command. It must not call os.Exit.
	Run func(ctx context.Context, args []string) error
}

// Argument documents a single positional argument of a command.
type Argument struct {
	// Name is the argument as written in Usage, e.g. "<host>".
	Name string
	// Help describes the argument.
	Help string
}
