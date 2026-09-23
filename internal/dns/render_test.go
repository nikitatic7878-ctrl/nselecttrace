package dns

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteTableHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, Result{Host: "example.com", Records: []Record{{TypeA, "93.184.216.34"}}}); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"HOST", "TYPE", "ADDRESS"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing column %q:\n%s", want, out)
		}
	}
}

func TestWriteTableRows(t *testing.T) {
	result := Result{
		Host: "example.com",
		Records: []Record{
			{TypeA, "93.184.216.34"},
			{TypeAAAA, "2606:2800:220:1:248:1893:25c8:1946"},
		},
		Duration: 24700 * time.Microsecond,
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, result); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"example.com", "A", "93.184.216.34",
		"AAAA", "2606:2800:220:1:248:1893:25c8:1946",
		"Resolved", "2 records", "Duration", "24.7ms",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// TestWriteTableDoesNotBracketIPv6 pins the deliberate difference from the
// endpoint tables: a resolved address is not an endpoint, so it has no port and
// must not be wrapped in square brackets.
func TestWriteTableDoesNotBracketIPv6(t *testing.T) {
	result := Result{Host: "example.com", Records: []Record{{TypeAAAA, "2606:2800::1"}}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, result); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "[2606:2800::1]") {
		t.Errorf("IPv6 address was bracketed although it is not an endpoint:\n%s", out)
	}
	if !strings.Contains(out, "2606:2800::1") {
		t.Errorf("IPv6 address is missing:\n%s", out)
	}
}

// TestWriteTableNoRecords is the honest-empty case: a successful resolution with
// nothing in it must read as an answer, not as a broken table.
func TestWriteTableNoRecords(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, Result{Host: "empty.example", Duration: 3 * time.Millisecond}); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "No records found") {
		t.Errorf("output does not report the empty answer:\n%s", out)
	}
	if !strings.Contains(out, "empty.example") {
		t.Errorf("output does not name the host:\n%s", out)
	}
	// The header must not be printed for an empty answer, or it would look like
	// a table that failed to populate.
	if strings.Contains(out, "HOST") {
		t.Errorf("an empty answer printed a table header:\n%s", out)
	}
}

func TestWriteTableMixedAnswerReportsBreakdown(t *testing.T) {
	result := Result{
		Host:     "example.com",
		Records:  []Record{{TypeA, "10.0.0.1"}, {TypeA, "10.0.0.2"}, {TypeAAAA, "::1"}},
		Duration: time.Millisecond,
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, result); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "3 records") {
		t.Errorf("output does not report the record count:\n%s", out)
	}
	if !strings.Contains(out, "2 A, 1 AAAA") {
		t.Errorf("output does not break the count down by family:\n%s", out)
	}
}

func TestWriteTableSingleRecordIsNotPluralised(t *testing.T) {
	result := Result{Host: "one.example", Records: []Record{{TypeA, "10.0.0.1"}}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, result); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "1 record") || strings.Contains(out, "1 records") {
		t.Errorf("output pluralises a single record incorrectly:\n%s", out)
	}
}

func TestWriteTableSingleFamilyOmitsBreakdown(t *testing.T) {
	result := Result{Host: "v4.example", Records: []Record{{TypeA, "10.0.0.1"}, {TypeA, "10.0.0.2"}}}

	var buf bytes.Buffer
	if err := WriteTable(&buf, result); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	if strings.Contains(buf.String(), "AAAA") {
		t.Errorf("a single-family answer mentions the other family:\n%s", buf.String())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "microseconds", d: 250 * time.Microsecond, want: "250µs"},
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

func TestPlural(t *testing.T) {
	if got := plural(0, "record"); got != "0 records" {
		t.Errorf("plural(0) = %q, want %q", got, "0 records")
	}
	if got := plural(1, "record"); got != "1 record" {
		t.Errorf("plural(1) = %q, want %q", got, "1 record")
	}
	if got := plural(2, "record"); got != "2 records" {
		t.Errorf("plural(2) = %q, want %q", got, "2 records")
	}
}
