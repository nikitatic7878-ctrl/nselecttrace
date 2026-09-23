package ping

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// placeholder is printed where a value is unavailable. A timed-out packet has no
// round-trip time, and showing a fake one would misreport the measurement.
const placeholder = "-"

// WriteTable renders the report as an aligned table followed by a summary.
//
// The table goes to w so that stdout carries only the result and stays
// pipe-friendly; nothing here writes to stderr. Addresses are printed bare — no
// square brackets — because an ICMP target is an address, not an endpoint, so
// there is no port for the brackets to disambiguate it from.
func WriteTable(w io.Writer, report Report) error {
	if err := writePackets(w, report.Result); err != nil {
		return err
	}
	return writeSummary(w, report)
}

// writePackets renders the HOST/ADDRESS/SEQ/STATUS/TIME table.
func writePackets(w io.Writer, r Result) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintf(tw, "HOST\tADDRESS\tSEQ\tSTATUS\tTIME\n"); err != nil {
		return err
	}

	for _, packet := range r.Packets {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
			r.Host,
			r.Address,
			packet.Sequence,
			packet.Status,
			packetTime(packet),
		); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// writeSummary renders the sent/received/lost counters.
//
// No min/avg/max is reported: the model deliberately does not keep per-packet
// statistics, and inventing aggregate numbers the run did not measure would be
// worse than omitting them.
func writeSummary(w io.Writer, report Report) error {
	r := report.Result

	_, err := fmt.Fprintf(w, "\nSent     : %d\nReceived : %d\nLost     : %d\n",
		r.Sent, r.Received, r.Lost())
	return err
}

// packetTime renders the round-trip time, or a placeholder when the packet was
// not measured.
func packetTime(p Packet) string {
	if !p.Succeeded() {
		return placeholder
	}
	return formatDuration(p.Duration)
}

// formatDuration renders a duration with a resolution that suits its magnitude:
// sub-millisecond, millisecond, or seconds.
//
// This mirrors what `nselecttrace dns` does. It is duplicated rather than
// extracted into a shared package on purpose: the two packages have no other
// reason to depend on each other, and a one-function internal helper package
// would be a new abstraction for its own sake.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
