package interfaces

import (
	"net"
	"sort"
)

// RawInterface pairs a system interface with the address strings reported for
// it. It is the seam between the operating system and the domain model: the
// only place in this package that touches net.Interface.
type RawInterface struct {
	Interface net.Interface
	// Addrs holds CIDR strings, mirroring net.Addr.String(), for example
	// "192.168.1.42/24". An empty slice means the addresses could not be read.
	Addrs []string
}

// Flag bits mirrored from the standard library so that the domain model does
// not depend on net.Flags values directly.
const (
	flagUp           = net.FlagUp
	flagLoopback     = net.FlagLoopback
	flagPointToPoint = net.FlagPointToPoint
)

// systemInterfaces reads the network interfaces of the local machine.
//
// It is a package variable so that tests can substitute a deterministic
// fixture without depending on the interfaces of the machine running them.
var systemInterfaces = readSystemInterfaces

func readSystemInterfaces() ([]RawInterface, error) {
	sys, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	raws := make([]RawInterface, 0, len(sys))
	for _, iface := range sys {
		raws = append(raws, RawInterface{
			Interface: iface,
			Addrs:     interfaceAddrStrings(iface),
		})
	}
	return raws, nil
}

// interfaceAddrStrings reads the addresses of one interface as CIDR strings.
// An unreadable address list yields nil; the caller reports the interface
// without addresses instead of failing.
func interfaceAddrStrings(iface net.Interface) []string {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}

	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}

// Sort orders ifs deterministically.
//
// The primary key is the interface index, which is assigned by the operating
// system and is therefore stable on a given machine (and, in practice, also
// across machines for the common interfaces). Ties are broken by name so the
// result never depends on the enumeration order of net.Interfaces.
func Sort(ifs []Interface) []Interface {
	sorted := make([]Interface, len(ifs))
	copy(sorted, ifs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Index != sorted[j].Index {
			return sorted[i].Index < sorted[j].Index
		}
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}
