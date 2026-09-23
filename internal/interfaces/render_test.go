package interfaces

import (
	"bytes"
	"strings"
	"testing"
)

// sample returns a deterministic fixture used by the rendering tests.
func sample() []Interface {
	return []Interface{
		{
			Index:        1,
			Name:         "Loopback Pseudo-Interface 1",
			Up:           true,
			Loopback:     true,
			MTU:          65536,
			HardwareAddr: "",
			Addresses: []Address{
				{IP: "127.0.0.1", CIDR: "127.0.0.1/8"},
				{IP: "::1", CIDR: "::1/128"},
			},
		},
		{
			Index:        12,
			Name:         "Ethernet",
			Up:           true,
			MTU:          1500,
			HardwareAddr: "00:11:22:33:44:55",
			Addresses: []Address{
				{IP: "192.168.1.42", CIDR: "192.168.1.42/24"},
				{IP: "fe80::1234", CIDR: "fe80::1234/64"},
			},
		},
		{
			Index:        7,
			Name:         "Wi-Fi",
			Up:           false,
			MTU:          1500,
			HardwareAddr: "66:77:88:99:aa:bb",
			Addresses:    nil,
		},
	}
}

func TestWriteTableHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, sample()); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6 (header + 2 + 2 + 1):\n%s", len(lines), out)
	}

	for _, want := range []string{"NAME", "STATE", "MAC", "ADDRESSES"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("header %q is missing column %q", lines[0], want)
		}
	}

	// One row per interface, plus a continuation row for the second address.
	for _, want := range []string{"Loopback Pseudo-Interface 1", "127.0.0.1/8", "::1/128", "Ethernet", "00:11:22:33:44:55", "192.168.1.42/24", "fe80::1234/64", "Wi-Fi"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}

	// IPv6 must not be dropped.
	if !strings.Contains(out, "::1/128") || !strings.Contains(out, "fe80::1234/64") {
		t.Errorf("IPv6 addresses were dropped:\n%s", out)
	}

	// Wi-Fi is down, has no addresses and must keep the address column empty.
	downRow := findRow(lines, "Wi-Fi")
	if downRow == "" {
		t.Fatalf("no row for Wi-Fi:\n%s", out)
	}
	if !strings.Contains(downRow, "DOWN") {
		t.Errorf("Wi-Fi row %q does not report DOWN", downRow)
	}
	if !strings.HasSuffix(downRow, placeholder) {
		t.Errorf("Wi-Fi row %q should end with the address placeholder %q", downRow, placeholder)
	}
}

func TestWriteTableAddressesAlignWithColumns(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, sample()); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	header := strings.Index(lines[0], "ADDRESSES")

	// Continuation lines must place their address in the address column.
	for _, line := range lines[1:] {
		if !strings.HasPrefix(strings.TrimSpace(line), "::1/128") &&
			!strings.HasPrefix(strings.TrimSpace(line), "fe80::1234/64") {
			continue
		}
		address := strings.Index(line, "::1/128")
		if address < 0 {
			address = strings.Index(line, "fe80::1234/64")
		}
		if address != header {
			t.Errorf("address column misaligned in %q: address at %d, header at %d", line, address, header)
		}
	}
}

func TestWriteTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, nil); err != nil {
		t.Fatalf("WriteTable returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "NAME") {
		t.Errorf("header row missing for empty input: %q", buf.String())
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Errorf("unexpected extra rows for empty input: %q", buf.String())
	}
}

func TestWriteDetails(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDetails(&buf, sample()); err != nil {
		t.Fatalf("WriteDetails returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Loopback Pseudo-Interface 1", "(loopback)",
		"Ethernet", "(regular)",
		"Wi-Fi",
		"index:", "state:", "mac:", "mtu:", "addresses:",
		"127.0.0.1/8", "::1/128", "fe80::1234/64",
		"UP", "DOWN",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("details output is missing %q:\n%s", want, out)
		}
	}

	// The interface without a MAC and without addresses shows placeholders.
	block := out[strings.Index(out, "Wi-Fi"):]
	if !strings.Contains(block, placeholder) {
		t.Errorf("Wi-Fi block has no placeholder:\n%s", block)
	}
}

func TestWriteDetailsPointToPoint(t *testing.T) {
	var buf bytes.Buffer
	ifs := []Interface{{Index: 9, Name: "tun0", Up: true, PointToPoint: true, MTU: 1420}}
	if err := WriteDetails(&buf, ifs); err != nil {
		t.Fatalf("WriteDetails returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "point-to-point") {
		t.Errorf("point-to-point flag not rendered:\n%s", buf.String())
	}
}

func findRow(lines []string, name string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, name) {
			return line
		}
	}
	return ""
}
