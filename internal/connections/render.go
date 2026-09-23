package connections

import (
	"fmt"
	"io"
	"text/tabwriter"

	"nselecttrace/internal/ports"
)

// Placeholders used where a value is unavailable. They match the ones used by
// `nselecttrace ports` so that both commands read the same way.
const (
	unknownProcess = "<unknown>"
	placeholder    = "-"
)

// WriteTable renders ps as an aligned plain-text table of local and remote
// endpoints.
//
// Endpoints are produced by ports.Port, which brackets IPv6 addresses, so a
// connection over IPv6 reads "[fe80::1]:52341" rather than an ambiguous string
// with three colons. Remote addresses are printed as addresses: nselecttrace never
// performs a reverse lookup, so what the socket table says is what is shown.
func WriteTable(w io.Writer, ps []ports.Port) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintf(tw, "PROTO\tLOCAL\tREMOTE\tSTATE\tPID\tPROCESS\n"); err != nil {
		return err
	}

	for _, p := range ps {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n",
			p.Protocol,
			p.LocalEndpoint(),
			p.RemoteEndpoint(),
			stateName(p),
			p.PID,
			processName(p),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// stateName renders the TCP state, or a placeholder for a protocol that has no
// state machine. A UDP connection therefore shows "-" rather than a fabricated
// ESTABLISHED.
func stateName(p ports.Port) string {
	if p.State == ports.StateNone {
		return placeholder
	}
	return p.State.String()
}

// processName renders the owning process, or a placeholder when it could not be
// resolved. A connection can outlive its owner, so an unresolved name is normal.
func processName(p ports.Port) string {
	if p.Process == "" {
		return unknownProcess
	}
	return p.Process
}
