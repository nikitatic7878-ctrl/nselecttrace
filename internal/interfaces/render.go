package interfaces

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// placeholder is printed where a value is unavailable.
const placeholder = "-"

// WriteTable renders ifs as an aligned plain-text table.
//
// The layout is deliberately spare: a header line, a rule and one row per
// interface. Addresses of the same interface are stacked in the address column
// so that a host with many addresses stays readable.
func WriteTable(w io.Writer, ifs []Interface) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintf(tw, "NAME\tSTATE\tMAC\tADDRESSES\n"); err != nil {
		return err
	}

	for _, iface := range ifs {
		addresses := addressLines(iface.Addresses)
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			orPlaceholder(iface.Name),
			state(iface),
			orPlaceholder(iface.HardwareAddr),
			addresses[0],
		); err != nil {
			return err
		}

		// Continuation rows leave the leading columns empty on purpose.
		for _, line := range addresses[1:] {
			if _, err := fmt.Fprintf(tw, "\t\t\t%s\n", line); err != nil {
				return err
			}
		}
	}

	return tw.Flush()
}

// WriteDetails renders ifs as a verbose, one-block-per-interface listing.
func WriteDetails(w io.Writer, ifs []Interface) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	for i, iface := range ifs {
		if i > 0 {
			if _, err := fmt.Fprintln(tw); err != nil {
				return err
			}
		}

		flags := flagList(iface)
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", iface.Name, flags); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  index:\t%d\n", iface.Index); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  state:\t%s\n", state(iface)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  mac:\t%s\n", orPlaceholder(iface.HardwareAddr)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  mtu:\t%d\n", iface.MTU); err != nil {
			return err
		}

		if len(iface.Addresses) == 0 {
			if _, err := fmt.Fprintf(tw, "  addresses:\t%s\n", placeholder); err != nil {
				return err
			}
			continue
		}
		for i, addr := range iface.Addresses {
			label := "  addresses:"
			if i > 0 {
				label = ""
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\n", label, addr); err != nil {
				return err
			}
		}
	}

	return tw.Flush()
}

// state renders the up/down state of an interface.
func state(iface Interface) string {
	if iface.Up {
		return "UP"
	}
	return "DOWN"
}

// flagList renders the interface kinds that are set.
func flagList(iface Interface) string {
	var flags []string
	if iface.Loopback {
		flags = append(flags, "loopback")
	}
	if iface.PointToPoint {
		flags = append(flags, "point-to-point")
	}
	if len(flags) == 0 {
		return "(regular)"
	}
	return "(" + strings.Join(flags, ", ") + ")"
}

// addressLines renders addresses one per line, or a placeholder when there are
// none.
func addressLines(addrs []Address) []string {
	if len(addrs) == 0 {
		return []string{placeholder}
	}

	lines := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		lines = append(lines, addr.String())
	}
	return lines
}

// orPlaceholder returns s, or the placeholder when s is empty.
func orPlaceholder(s string) string {
	if s == "" {
		return placeholder
	}
	return s
}
