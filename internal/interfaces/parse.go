package interfaces

import (
	"net"
	"strconv"
	"strings"
)

// ParseAddress converts a CIDR string as returned by net.Interface.Addrs into
// an Address. Both 192.168.1.42/24 and fe80::1/64 are accepted.
func ParseAddress(cidr string) (Address, error) {
	ip, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return Address{}, err
	}

	// net.ParseCIDR keeps the address as it was written, so ip.String() is the
	// host address ("192.168.1.42") rather than the network address
	// ("192.168.1.0"). The prefix length comes from the parsed mask.
	host := ip.String()

	return Address{
		IP:   host,
		CIDR: host + "/" + prefixLength(ipNet),
	}, nil
}

// ParseIP converts a bare IP string into an Address without prefix length.
func ParseIP(ip string) (Address, error) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return Address{}, &net.AddrError{Err: "invalid IP address", Addr: ip}
	}
	return Address{IP: parsed.String()}, nil
}

// IsIPv6IP reports whether s is an IPv6 address.
func IsIPv6IP(s string) bool {
	ip := net.ParseIP(strings.TrimSpace(s))
	return ip != nil && ip.To4() == nil
}

// IsIPv6CIDR reports whether the CIDR string contains an IPv6 address.
func IsIPv6CIDR(cidr string) bool {
	ip, _, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return false
	}
	return ip.To4() == nil
}

// prefixLength renders the prefix of a parsed network as a decimal string.
func prefixLength(n *net.IPNet) string {
	ones, _ := n.Mask.Size()
	return strconv.Itoa(ones)
}

// FormatHardwareAddr renders a hardware address in the conventional
// colon-separated lower case form, or "" when there is none.
//
// net.HardwareAddr.String() is deliberately avoided: on Windows it can produce
// an all-zero address for interfaces that do not have one (for example the
// loopback pseudo-interface), and an all-zero address is not useful to a
// human.
func FormatHardwareAddr(addr net.HardwareAddr) string {
	if len(addr) == 0 || isZeroAddr(addr) {
		return ""
	}
	const hex = "0123456789abcdef"
	var b strings.Builder
	b.Grow(len(addr)*3 - 1)
	for i, octet := range addr {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteByte(hex[octet>>4])
		b.WriteByte(hex[octet&0x0f])
	}
	return b.String()
}

// isZeroAddr reports whether every octet of addr is zero.
func isZeroAddr(addr net.HardwareAddr) bool {
	for _, octet := range addr {
		if octet != 0 {
			return false
		}
	}
	return true
}
