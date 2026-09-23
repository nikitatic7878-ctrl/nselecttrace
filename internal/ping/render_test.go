package ping

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func okPacket(sequence int, d time.Duration) Packet {
	return Packet{Sequence: sequence, Status: StatusOK, Duration: d}
}

func TestWriteTableHeader(t *testing.T) {
	var buf bytes.Buffer
	report := Report{Result: Result{
		Host: "localhost", Address: "127.0.0.1",
		Packets: []Packet{okPacket(1, time.Millisecond)}, Sent: 1, Received: 1,
	}}

	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"HOST", "ADDRESS", "SEQ", "STATUS", "TIME"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing column %q:\n%s", want, out)
		}
	}
}

func TestWriteTableRows(t *testing.T) {
	report := Report{
		Result: Result{
			Host:     "localhost",
			Address:  "127.0.0.1",
			Packets:  []Packet{okPacket(1, 82*time.Microsecond), okPacket(2, 71*time.Microsecond)},
			Sent:     2,
			Received: 2,
		},
		Duration: 5 * time.Millisecond,
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"localhost", "127.0.0.1", "OK", "82µs", "71µs", "Sent", "Received", "Lost"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// TestWriteTableTimeoutRowHasNoFakeTime pins that an unmeasured packet shows a
// placeholder instead of an invented round-trip time.
func TestWriteTableTimeoutRowHasNoFakeTime(t *testing.T) {
	report := Report{Result: Result{
		Host: "192.0.2.1", Address: "192.0.2.1",
		Packets: []Packet{{Sequence: 1, Status: StatusTimeout}}, Sent: 1,
	}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	row := rowFor(out, "TIMEOUT")
	if row == "" {
		t.Fatalf("timed-out packet is not labelled TIMEOUT:\n%s", out)
	}
	if !strings.Contains(row, placeholder) {
		t.Errorf("timed-out row has no time placeholder: %q", row)
	}
	for _, unit := range []string{"µs", "ms"} {
		if strings.Contains(row, unit) {
			t.Errorf("timed-out row contains a fabricated time (%s): %q", unit, row)
		}
	}
}

func TestWriteTableErrorRowHasNoFakeTime(t *testing.T) {
	report := Report{Result: Result{
		Host: "127.0.0.1", Address: "127.0.0.1",
		Packets: []Packet{{Sequence: 1, Status: StatusError, Err: errTest}}, Sent: 1,
	}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	row := rowFor(out, "ERROR")
	if row == "" {
		t.Fatalf("failed packet is not labelled ERROR:\n%s", out)
	}
	if !strings.Contains(row, placeholder) {
		t.Errorf("failed row has no time placeholder: %q", row)
	}
}

// TestWriteTableZeroReceived is the all-lost case: the summary must be honest.
func TestWriteTableZeroReceived(t *testing.T) {
	report := Report{Result: Result{
		Host: "192.0.2.1", Address: "192.0.2.1",
		Packets: []Packet{{Sequence: 1, Status: StatusTimeout}, {Sequence: 2, Status: StatusTimeout}},
		Sent:    2, Received: 0,
	}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Received : 0") {
		t.Errorf("summary does not report zero received:\n%s", out)
	}
	if !strings.Contains(out, "Lost     : 2") {
		t.Errorf("summary does not report two lost:\n%s", out)
	}
}

func TestWriteTablePartialLossSummary(t *testing.T) {
	report := Report{Result: Result{
		Host: "127.0.0.1", Address: "127.0.0.1",
		Packets: []Packet{
			okPacket(1, time.Millisecond),
			{Sequence: 2, Status: StatusTimeout},
			okPacket(3, time.Millisecond),
		},
		Sent: 3, Received: 2,
	}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"Sent     : 3", "Received : 2", "Lost     : 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}
}
