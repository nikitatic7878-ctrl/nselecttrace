package ports

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// Placeholders used where a value is unavailable.
//
// unknownProcess is the single format used for an unresolved owner: on Windows
// and macOS another user's process cannot be inspected without privileges, so
// the situation is normal and must read the same everywhere.
const (
	placeholder    = "-"
	unknownProcess = "<unknown>"
)

// WriteTable renders ps as an aligned plain-text table.
//
// The layout stays spare on purpose: a header and one row per socket. Remote
// endpoints are collected into the domain model but not printed here, because
// this command lists listening and bound sockets; `nselecttrace connections` will
// render them.
func WriteTable(w io.Writer, ps []Port) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintf(tw, "PROTO\tLOCAL ADDRESS\tSTATE\tPID\tPROCESS\n"); err != nil {
		return err
	}

	for _, p := range ps {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			p.Protocol,
			p.LocalEndpoint(),
			p.State,
			p.PID,
			processName(p),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// processName renders the owning process, or a placeholder when it could not be
// resolved.
func processName(p Port) string {
	if p.Process == "" {
		return unknownProcess
	}
	return p.Process
}
