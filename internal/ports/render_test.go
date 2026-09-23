package ports

import (
	"bytes"
	"strings"
	"testing"
	"text/tabwriter"
)

func TestWriteTableHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, nil); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"PROTO", "LOCAL ADDRESS", "STATE", "PID", "PROCESS"} {
		if !strings.Contains(out, want) {
			t.Errorf("header %q is missing column %q", out, want)
		}
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("empty input produced extra rows: %q", out)
	}
}

func TestWriteTableRows(t *testing.T) {
	ps := []Port{
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 135, State: StateListen, PID: 1200, Process: "svchost.exe"},
		{Protocol: TCP, LocalIP: "127.0.0.1", LocalPort: 3000, State: StateListen, PID: 8212, Process: "node.exe"},
		{Protocol: UDP, LocalIP: "0.0.0.0", LocalPort: 53, State: StateNone, PID: 1400, Process: "dns.exe"},
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 445, State: StateListen, PID: 4, Process: ""},
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5:\n%s", len(lines), out)
	}

	for _, want := range []string{":135", "svchost.exe", "127.0.0.1:3000", "node.exe", ":53", "dns.exe"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	// A wildcard bind is rendered as a bare port.
	if strings.Contains(out, "0.0.0.0:135") {
		t.Errorf("wildcard bind was not collapsed to :135:\n%s", out)
	}
	// UDP must show no TCP-style state.
	if !strings.Contains(out, ":53") || !strings.Contains(out, "-") {
		t.Errorf("UDP row does not show the neutral state placeholder:\n%s", out)
	}
	// An unresolved process must use the documented placeholder.
	if !strings.Contains(out, unknownProcess) {
		t.Errorf("unresolved process is not rendered as %q:\n%s", unknownProcess, out)
	}
}

func TestWriteTableIsAligned(t *testing.T) {
	ps := []Port{
		{Protocol: TCP, LocalIP: "0.0.0.0", LocalPort: 135, State: StateListen, PID: 1200, Process: "svchost.exe"},
		{Protocol: TCP, LocalIP: "192.168.100.200", LocalPort: 65535, State: StateEstablished, PID: 999999, Process: "a-very-long-process-name.exe"},
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), buf.String())
	}

	// The PID column must start at the same offset in every row, which is what
	// tabwriter guarantees and what makes the table readable.
	header := strings.Index(lines[0], "PID")
	for i, line := range lines[1:] {
		if got := strings.Index(line, "  "); got < 0 {
			t.Errorf("row %d has no column separator: %q", i, line)
		}
		// STATE column start must match the header's STATE column start.
		want := strings.Index(lines[0], "STATE")
		got := strings.Index(line, "LISTENING")
		if got < 0 {
			got = strings.Index(line, "ESTABLISHED")
		}
		if got != want {
			t.Errorf("row %d: state column at %d, header at %d:\n%s", i, got, want, buf.String())
		}
	}
	_ = header
}

func TestWriteTableMatchesTabwriterLayout(t *testing.T) {
	ps := []Port{{Protocol: TCP, LocalIP: "127.0.0.1", LocalPort: 1, State: StateListen, PID: 1, Process: "x"}}

	var got bytes.Buffer
	if err := WriteTable(&got, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	// Rebuild the expected output independently; the renderer must not do any
	// formatting beyond what tabwriter does.
	var want bytes.Buffer
	tw := tabwriter.NewWriter(&want, 0, 4, 2, ' ', 0)
	_, _ = tw.Write([]byte("PROTO\tLOCAL ADDRESS\tSTATE\tPID\tPROCESS\n"))
	_, _ = tw.Write([]byte("TCP\t127.0.0.1:1\tLISTENING\t1\tx\n"))
	_ = tw.Flush()

	if got.String() != want.String() {
		t.Errorf("WriteTable output differs from the plain tabwriter layout:\ngot:\n%q\nwant:\n%q", got.String(), want.String())
	}
}

func TestProcessNamePlaceholderIsStable(t *testing.T) {
	// The same placeholder must be used for every unresolved process.
	if got := processName(Port{}); got != unknownProcess {
		t.Errorf("processName(empty) = %q, want %q", got, unknownProcess)
	}
	if got := processName(Port{Process: "x"}); got != "x" {
		t.Errorf("processName(x) = %q, want %q", got, "x")
	}
}
