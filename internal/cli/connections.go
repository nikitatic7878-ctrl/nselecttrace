package cli

import (
	"context"
	"fmt"

	"nselecttrace/internal/connections"
)

// connectionsCommand lists the active network connections of the local machine.
//
// A connection is a socket with a remote endpoint, so the data comes from
// internal/ports (through internal/connections) rather than from a second
// platform source. This file only parses flags and reports errors.
func (a *App) connectionsCommand() Command {
	return Command{
		Name:    "connections",
		Summary: "list active connections with their remote endpoints",
		Usage:   "connections [--tcp] [--udp] [--port <port>] [--pid <pid>] [--process <name>] [--state <state>] [--remote-port <port>]",
		Arguments: []Argument{
			{Name: "--tcp", Help: "show TCP connections only"},
			{Name: "--udp", Help: "show UDP connections only"},
			{Name: "--port <port>", Help: "show only connections using this local port (1-65535)"},
			{Name: "--remote-port <port>", Help: "show only connections to this remote port (1-65535)"},
			{Name: "--pid <pid>", Help: "show only connections owned by this process ID"},
			{Name: "--process <name>", Help: "show only connections owned by a matching process name"},
			{Name: "--state <state>", Help: "show only this TCP state, e.g. ESTABLISHED or TIME_WAIT"},
		},
		Run: a.runConnections,
	}
}

func (a *App) runConnections(ctx context.Context, args []string) error {
	_ = ctx

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
		return err
	}
	// Every connection has a remote endpoint by definition, so the selection is
	// expressed once in the filter rather than by a separate code path.
	base.RemoteOnly = true

	filter := connections.Filter{Filter: base, RemotePort: remotePort}

	all, err := connections.List()
	if err != nil {
		return WithCode(1, err)
	}

	selected := connections.Apply(all, filter)
	if err := connections.WriteTable(a.stdout, selected); err != nil {
		return WithCode(1, fmt.Errorf("failed to write output: %w", err))
	}

	if unknown := connections.CountUnknownProcesses(selected); unknown > 0 {
		fmt.Fprintf(a.stderr, "nselecttrace: %d of %d connections have no readable process name "+
			"(reading another user's process requires elevated privileges)\n",
			unknown, len(selected))
	}
	return nil
}
