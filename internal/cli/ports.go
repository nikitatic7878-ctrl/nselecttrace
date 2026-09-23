package cli

import (
	"context"
	"fmt"

	"nselecttrace/internal/ports"
)

// portsCommand lists the listening and bound sockets of the local machine.
//
// All collection, filtering and rendering lives in internal/ports; this file
// only parses flags, maps them onto a ports.Filter and reports errors through
// the shared exit-code model.
func (a *App) portsCommand() Command {
	return Command{
		Name:    "ports",
		Summary: "list listening and bound TCP/UDP ports with their processes",
		Usage:   "ports [--tcp] [--udp] [--port <port>] [--pid <pid>] [--process <name>] [--state <state>]",
		Arguments: []Argument{
			{Name: "--tcp", Help: "show TCP sockets only"},
			{Name: "--udp", Help: "show UDP sockets only"},
			{Name: "--port <port>", Help: "show only this local port (1-65535)"},
			{Name: "--pid <pid>", Help: "show only sockets owned by this process ID"},
			{Name: "--process <name>", Help: "show only sockets owned by a matching process name"},
			{Name: "--state <state>", Help: "show only this TCP state, e.g. LISTENING or ESTABLISHED"},
		},
		Run: a.runPorts,
	}
}

func (a *App) runPorts(ctx context.Context, args []string) error {
	_ = ctx

	filter, err := parsePortsFlags("ports", args, nil)
	if err != nil {
		return err
	}

	all, err := ports.List()
	if err != nil {
		return WithCode(1, err)
	}

	selected := ports.Apply(all, filter)
	if err := ports.WriteTable(a.stdout, selected); err != nil {
		return WithCode(1, fmt.Errorf("failed to write output: %w", err))
	}

	// Process names are best effort: report how much attribution was possible
	// instead of silently printing blanks. This goes to stderr so that the
	// table itself stays pipeable.
	if unknown := ports.CountUnknownProcesses(selected); unknown > 0 {
		fmt.Fprintf(a.stderr, "nselecttrace: %d of %d sockets have no readable process name "+
			"(reading another user's process requires elevated privileges)\n",
			unknown, len(selected))
	}
	return nil
}
