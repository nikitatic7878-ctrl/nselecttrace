package connections

import (
	"bytes"
	"strings"
	"testing"

	"nselecttrace/internal/ports"
)

func TestWriteTableHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, nil); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"PROTO", "LOCAL", "REMOTE", "STATE", "PID", "PROCESS"} {
		if !strings.Contains(out, want) {
			t.Errorf("header %q is missing column %q", out, want)
		}
	}
	if strings.Count(out, "\n") != 1 {
		t.Errorf("empty input produced extra rows: %q", out)
	}
}

func TestWriteTableRendersEndpoints(t *testing.T) {
	ps := []ports.Port{
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 52341,
			RemoteIP: "142.250.74.14", RemotePort: 443,
			State: ports.StateEstablished, PID: 8416, Process: "chrome.exe"},
		{Protocol: ports.TCP, LocalIP: "192.168.1.6", LocalPort: 53122,
			RemoteIP: "192.168.1.10", RemotePort: 22,
			State: ports.StateTimeWait, PID: 0, Process: ""},
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"192.168.1.6:52341", "142.250.74.14:443", "ESTABLISHED", "8416", "chrome.exe",
		"192.168.1.6:53122", "192.168.1.10:22", "TIME_WAIT", unknownProcess,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// TestWriteTableBracketsIPv6 is the formatting guard: an unbracketed IPv6
// endpoint would be ambiguous, because the address itself contains colons.
func TestWriteTableBracketsIPv6(t *testing.T) {
	ps := []ports.Port{
		{Protocol: ports.TCP, LocalIP: "fe80::1234", LocalPort: 52341,
			RemoteIP: "2607:f8b0::1", RemotePort: 443,
			State: ports.StateEstablished, PID: 8416, Process: "chrome.exe"},
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "[fe80::1234]:52341") {
		t.Errorf("local IPv6 endpoint is not bracketed:\n%s", out)
	}
	if !strings.Contains(out, "[2607:f8b0::1]:443") {
		t.Errorf("remote IPv6 endpoint is not bracketed:\n%s", out)
	}
	// The naive concatenation must not appear.
	if strings.Contains(out, "fe80::1234:52341") {
		t.Errorf("output contains an ambiguous IPv6 endpoint:\n%s", out)
	}
}

func TestWriteTableUDPShowsNoState(t *testing.T) {
	ps := []ports.Port{
		{Protocol: ports.UDP, LocalIP: "192.168.1.6", LocalPort: 53000,
			RemoteIP: "8.8.8.8", RemotePort: 53,
			State: ports.StateNone, PID: 8212, Process: "app.exe"},
	}

	var buf bytes.Buffer
	if err := WriteTable(&buf, ps); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "ESTABLISHED") || strings.Contains(out, "LISTENING") {
		t.Errorf("a UDP connection was given a TCP state:\n%s", out)
	}
	if !strings.Contains(out, placeholder) {
		t.Errorf("UDP connection has no neutral state placeholder:\n%s", out)
	}
}

func TestStateName(t *testing.T) {
	if got := stateName(ports.Port{State: ports.StateNone}); got != placeholder {
		t.Errorf("stateName(StateNone) = %q, want %q", got, placeholder)
	}
	if got := stateName(ports.Port{State: ports.StateEstablished}); got != "ESTABLISHED" {
		t.Errorf("stateName(ESTABLISHED) = %q, want ESTABLISHED", got)
	}
	if got := stateName(ports.Port{State: ports.StateTimeWait}); got != "TIME_WAIT" {
		t.Errorf("stateName(TIME_WAIT) = %q, want TIME_WAIT", got)
	}
}

func TestProcessNamePlaceholderIsStable(t *testing.T) {
	// The placeholder must match the one `nselecttrace ports` uses, so the two
	// commands read the same way.
	if got := processName(ports.Port{}); got != unknownProcess {
		t.Errorf("processName(empty) = %q, want %q", got, unknownProcess)
	}
	if got := processName(ports.Port{Process: "x"}); got != "x" {
		t.Errorf("processName(x) = %q, want %q", got, "x")
	}
	if unknownProcess != "<unknown>" {
		t.Errorf("unknownProcess = %q, want %q", unknownProcess, "<unknown>")
	}
}
