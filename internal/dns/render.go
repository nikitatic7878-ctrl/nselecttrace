package dns

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// WriteTable renders r as an aligned table followed by a short summary.
//
// The table goes to w so that stdout carries only the result and stays
// pipe-friendly; nothing here writes to stderr. Addresses are printed bare —
// no square brackets — because a resolved address is not an endpoint, so there
// is no port to disambiguate it from.
func WriteTable(w io.Writer, r Result) error {
	if err := writeRecords(w, r); err != nil {
		return err
	}
	return writeSummary(w, r)
}

// writeRecords renders the HOST/TYPE/ADDRESS table.
func writeRecords(w io.Writer, r Result) error {
	if r.Empty() {
		// A successful resolution that produced nothing is a real answer, not
		// a failure, and must not be dressed up as an empty table.
		_, err := fmt.Fprintf(w, "No records found for %s.\n", r.Host)
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if _, err := fmt.Fprintf(tw, "HOST\tTYPE\tADDRESS\n"); err != nil {
		return err
	}
	for _, rec := range r.Records {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Host, rec.Type, rec.Address); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// writeSummary renders the record count and the resolution time.
func writeSummary(w io.Writer, r Result) error {
	a, aaaa := r.Counts()

	_, err := fmt.Fprintf(w, "\nResolved  : %s%s\nDuration  : %s\n",
		plural(len(r.Records), "record"),
		breakdown(a, aaaa),
		formatDuration(r.Duration),
	)
	return err
}

// breakdown describes a mixed answer, for example " (2 A, 1 AAAA)". It stays
// empty for a single-family answer so the common case reads plainly.
func breakdown(a, aaaa int) string {
	if a > 0 && aaaa > 0 {
		return fmt.Sprintf(" (%d A, %d AAAA)", a, aaaa)
	}
	return ""
}

// plural renders count with a noun, pluralised by appending "s".
func plural(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

// formatDuration renders a resolution time with a resolution that suits its
// magnitude: sub-millisecond, millisecond, or seconds.
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
