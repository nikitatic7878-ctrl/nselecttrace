// Package interfaces collects information about the network interfaces of the
// local machine.
//
// The package returns a plain domain model so that the CLI, a future TUI and
// JSON output can all consume the same values. It deliberately exposes no
// net.Interface to its callers.
package interfaces

import "fmt"

// Interface describes a single network interface of the local machine.
type Interface struct {
	Index        int
	Name         string
	HardwareAddr string // empty when the interface has no MAC address
	Up           bool   // administrative/operational state is up
	Loopback     bool   // loopback interface
	PointToPoint bool   // point-to-point link (for example a VPN tunnel)
	MTU          int
	Addresses    []Address
}

// Address is a single address assigned to an interface.
type Address struct {
	IP   string // bare IP, for example "192.168.1.42" or "fe80::1"
	CIDR string // address with prefix length, for example "192.168.1.42/24"
}

// IsIPv6 reports whether the address is an IPv6 address.
func (a Address) IsIPv6() bool {
	if a.CIDR != "" {
		return IsIPv6CIDR(a.CIDR)
	}
	return IsIPv6IP(a.IP)
}

// String implements fmt.Stringer, preferring the address with prefix length.
func (a Address) String() string {
	if a.CIDR != "" {
		return a.CIDR
	}
	return a.IP
}

// List returns the network interfaces of the local machine, ordered by
// ascending interface index. The order is stable across runs and does not
// depend on the operating system's enumeration order.
//
// A failure to read the addresses of a single interface is not fatal: the
// interface is reported without addresses rather than failing the whole call.
func List() ([]Interface, error) {
	raws, err := systemInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}
	return FromSystem(raws), nil
}

// FromSystem converts raw system interfaces into the domain model. It is
// exported so that the conversion can be exercised without touching the
// network stack of the running machine.
func FromSystem(raws []RawInterface) []Interface {
	out := make([]Interface, 0, len(raws))
	for _, raw := range raws {
		out = append(out, newInterface(raw))
	}
	return Sort(out)
}

// newInterface maps one RawInterface onto the domain model.
func newInterface(raw RawInterface) Interface {
	addresses := make([]Address, 0, len(raw.Addrs))
	for _, cidr := range raw.Addrs {
		address, err := ParseAddress(cidr)
		if err != nil {
			// The kernel reported something we cannot represent; skipping the
			// address is better than dropping the whole interface.
			continue
		}
		addresses = append(addresses, address)
	}

	flags := raw.Interface.Flags
	return Interface{
		Index:        raw.Interface.Index,
		Name:         raw.Interface.Name,
		HardwareAddr: FormatHardwareAddr(raw.Interface.HardwareAddr),
		Up:           flags&flagUp != 0,
		Loopback:     flags&flagLoopback != 0,
		PointToPoint: raw.Interface.Flags&flagPointToPoint != 0,
		MTU:          raw.Interface.MTU,
		Addresses:    addresses,
	}
}
