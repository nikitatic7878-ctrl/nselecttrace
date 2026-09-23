package ping

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

var errTest = errors.New("test failure")

// TestWriteTableDoesNotBracketIPv6 pins the deliberate difference from the
// endpoint tables: an ICMP target is an address, not an endpoint, so it has no
// port and must not be wrapped in square brackets.
func TestWriteTableDoesNotBracketIPv6(t *testing.T) {
	report := Report{Result: Result{
		Host: "::1", Address: "::1",
		Packets: []Packet{okPacket(1, time.Microsecond)}, Sent: 1, Received: 1,
	}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "[::1]") {
		t.Errorf("IPv6 address was bracketed although it is not an endpoint:\n%s", out)
	}
	if !strings.Contains(out, "::1") {
		t.Errorf("IPv6 address is missing:\n%s", out)
	}
}

func TestWriteTableEmptyPackets(t *testing.T) {
	// Defensive: a report with no packets must still render a header and a
	// summary rather than panicking.
	var buf bytes.Buffer
	report := Report{Result: Result{Host: "localhost", Address: "127.0.0.1"}}

	if err := WriteTable(&buf, report); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "HOST") {
		t.Errorf("header is missing:\n%s", out)
	}
	if !strings.Contains(out, "Sent     : 0") {
		t.Errorf("summary is missing:\n%s", out)
	}
}

func TestWriteTablePropagatesWriterFailure(t *testing.T) {
	report := Report{Result: Result{
		Host: "localhost", Address: "127.0.0.1",
		Packets: []Packet{okPacket(1, time.Microsecond)}, Sent: 1, Received: 1,
	}}

	if err := WriteTable(failingWriter{}, report); err == nil {
		t.Fatal("WriteTable returned nil error for a failing writer")
	}
}

// failingWriter rejects every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestPacketTime(t *testing.T) {
	if got := packetTime(okPacket(1, 82*time.Microsecond)); got != "82µs" {
		t.Errorf("packetTime(ok) = %q, want 82µs", got)
	}
	if got := packetTime(Packet{Status: StatusTimeout}); got != placeholder {
		t.Errorf("packetTime(timeout) = %q, want %q", got, placeholder)
	}
	if got := packetTime(Packet{Status: StatusError}); got != placeholder {
		t.Errorf("packetTime(error) = %q, want %q", got, placeholder)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "microseconds", d: 82 * time.Microsecond, want: "82µs"},
		{name: "sub-millisecond", d: 900 * time.Microsecond, want: "900µs"},
		{name: "milliseconds", d: 24700 * time.Microsecond, want: "24.7ms"},
		{name: "seconds", d: 1500 * time.Millisecond, want: "1.50s"},
		{name: "zero", d: 0, want: "0µs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDuration(tt.d); got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusOK, "OK"},
		{StatusTimeout, "TIMEOUT"},
		{StatusError, "ERROR"},
		{Status(0), "UNKNOWN"},
		{Status(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestPacketSucceeded(t *testing.T) {
	if !okPacket(1, time.Millisecond).Succeeded() {
		t.Error("an OK packet reports Succeeded() = false")
	}
	if (Packet{Status: StatusTimeout}).Succeeded() {
		t.Error("a timed-out packet reports Succeeded() = true")
	}
	if (Packet{Status: StatusError}).Succeeded() {
		t.Error("a failed packet reports Succeeded() = true")
	}
}

// rowFor returns the first rendered row containing marker.
func rowFor(out, marker string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	return ""
}
