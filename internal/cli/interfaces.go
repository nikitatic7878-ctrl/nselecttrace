package cli

import (
	"context"
	"fmt"

	"nselecttrace/internal/interfaces"
)

// interfacesCommand lists the network interfaces of the local machine.
//
// All data collection and rendering lives in internal/interfaces; this file
// only wires the command into the CLI, parses its flags and maps errors onto
// the shared exit-code model.
func (a *App) interfacesCommand() Command {
	return Command{
		Name:    "interfaces",
		Summary: "list network interfaces and their addresses",
		Usage:   "interfaces [--verbose]",
		Run:     a.runInterfaces,
	}
}

func (a *App) runInterfaces(ctx context.Context, args []string) error {
	_ = ctx

	var verbose bool
	for _, arg := range args {
		switch arg {
		case "--verbose", "-v":
			verbose = true
		default:
			return UsageError("interfaces: unknown argument %q", arg)
		}
	}

	ifs, err := interfaces.List()
	if err != nil {
		return WithCode(1, err)
	}

	if verbose {
		err = interfaces.WriteDetails(a.stdout, ifs)
	} else {
		err = interfaces.WriteTable(a.stdout, ifs)
	}
	if err != nil {
		return WithCode(1, fmt.Errorf("failed to write output: %w", err))
	}
	return nil
}
