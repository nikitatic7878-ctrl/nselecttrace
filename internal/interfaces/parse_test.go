package interfaces

import (
	"net"
	"testing"
)

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		wantIP   string
		wantCIDR string
		wantIPv6 bool
	}{
		{name: "ipv4 private", cidr: "192.168.1.42/24", wantIP: "192.168.1.42", wantCIDR: "192.168.1.42/24"},
		{name: "ipv4 with leading zeros in prefix", cidr: "10.0.0.5/8", wantIP: "10.0.0.5", wantCIDR: "10.0.0.5/8"},
		{name: "ipv4 host prefix", cidr: "172.16.5.7/32", wantIP: "172.16.5.7", wantCIDR: "172.16.5.7/32"},
		{name: "ipv4 network address", cidr: "10.0.0.0/8", wantIP: "10.0.0.0", wantCIDR: "10.0.0.0/8"},
		{name: "loopback ipv4", cidr: "127.0.0.1/8", wantIP: "127.0.0.1", wantCIDR: "127.0.0.1/8"},
		{name: "loopback ipv6", cidr: "::1/128", wantIP: "::1", wantCIDR: "::1/128", wantIPv6: true},
		{name: "ipv6 link local", cidr: "fe80::1234/64", wantIP: "fe80::1234", wantCIDR: "fe80::1234/64", wantIPv6: true},
		{name: "ipv6 global", cidr: "2001:db8::dead:beef/64", wantIP: "2001:db8::dead:beef", wantCIDR: "2001:db8::dead:beef/64", wantIPv6: true},
		{name: "ipv6 host prefix", cidr: "2001:db8::1/128", wantIP: "2001:db8::1", wantCIDR: "2001:db8::1/128", wantIPv6: true},
		{name: "surrounding whitespace", cidr: "  192.168.0.1/16\t", wantIP: "192.168.0.1", wantCIDR: "192.168.0.1/16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddress(tt.cidr)
			if err != nil {
				t.Fatalf("ParseAddress(%q) returned error: %v", tt.cidr, err)
			}
			if got.IP != tt.wantIP {
				t.Errorf("IP = %q, want %q", got.IP, tt.wantIP)
			}
			if got.CIDR != tt.wantCIDR {
				t.Errorf("CIDR = %q, want %q", got.CIDR, tt.wantCIDR)
			}
			if got.IsIPv6() != tt.wantIPv6 {
				t.Errorf("IsIPv6() = %v, want %v", got.IsIPv6(), tt.wantIPv6)
			}
			if got.String() != tt.wantCIDR {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantCIDR)
			}
		})
	}
}

func TestParseAddressRejectsMalformedInput(t *testing.T) {
	malformed := []string{
		"",
		"   ",
		"not-an-address",
		"192.168.1.42",
		"192.168.1.42/",
		"192.168.1.42/33",
		"192.168.1.42/-1",
		"192.168.1.999/24",
		"fe80::1/129",
		"1.2.3.4/24/24",
	}

	for _, input := range malformed {
		t.Run(input, func(t *testing.T) {
			if addr, err := ParseAddress(input); err == nil {
				t.Errorf("ParseAddress(%q) = %+v, want error", input, addr)
			}
		})
	}
}

func TestParseIP(t *testing.T) {
	tests := []struct {
		in       string
		wantIP   string
		wantIPv6 bool
	}{
		{in: "192.168.1.42", wantIP: "192.168.1.42"},
		{in: "::1", wantIP: "::1", wantIPv6: true},
		{in: "fe80::1234", wantIP: "fe80::1234", wantIPv6: true},
		{in: " 10.0.0.1 ", wantIP: "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseIP(tt.in)
			if err != nil {
				t.Fatalf("ParseIP(%q) returned error: %v", tt.in, err)
			}
			if got.IP != tt.wantIP {
				t.Errorf("IP = %q, want %q", got.IP, tt.wantIP)
			}
			if got.CIDR != "" {
				t.Errorf("CIDR = %q, want empty", got.CIDR)
			}
			if got.String() != tt.wantIP {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantIP)
			}
			if got.IsIPv6() != tt.wantIPv6 {
				t.Errorf("IsIPv6() = %v, want %v", got.IsIPv6(), tt.wantIPv6)
			}
		})
	}
}

func TestParseIPRejectsMalformedInput(t *testing.T) {
	for _, input := range []string{"", "1.2.3.4/24", "not-an-ip", "300.1.1.1"} {
		t.Run(input, func(t *testing.T) {
			if addr, err := ParseIP(input); err == nil {
				t.Errorf("ParseIP(%q) = %+v, want error", input, addr)
			}
		})
	}
}

func TestIsIPv6CIDRAndIP(t *testing.T) {
	if IsIPv6CIDR("::1/128") != true {
		t.Error(`IsIPv6CIDR("::1/128") = false, want true`)
	}
	if IsIPv6CIDR("127.0.0.1/8") != false {
		t.Error(`IsIPv6CIDR("127.0.0.1/8") = true, want false`)
	}
	if IsIPv6CIDR("garbage") != false {
		t.Error(`IsIPv6CIDR("garbage") = true, want false`)
	}
	if IsIPv6IP("fe80::1") != true {
		t.Error(`IsIPv6IP("fe80::1") = false, want true`)
	}
	if IsIPv6IP("8.8.8.8") != false {
		t.Error(`IsIPv6IP("8.8.8.8") = true, want false`)
	}
	if IsIPv6IP("garbage") != false {
		t.Error(`IsIPv6IP("garbage") = true, want false`)
	}
}

func TestFormatHardwareAddr(t *testing.T) {
	tests := []struct {
		name string
		in   net.HardwareAddr
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "empty", in: net.HardwareAddr{}, want: ""},
		{name: "all zero", in: net.HardwareAddr{0, 0, 0, 0, 0, 0}, want: ""},
		{name: "typical", in: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}, want: "00:11:22:33:44:55"},
		{name: "uppercase input rendered lowercase", in: net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, want: "aa:bb:cc:dd:ee:ff"},
		{name: "eui-64", in: net.HardwareAddr{0x02, 0x00, 0x5E, 0x10, 0, 1, 2, 3}, want: "02:00:5e:10:00:01:02:03"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatHardwareAddr(tt.in); got != tt.want {
				t.Errorf("FormatHardwareAddr(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
