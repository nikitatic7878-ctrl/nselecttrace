package interfaces

import (
	"errors"
	"net"
	"strings"
	"testing"
)

// fixture builds a RawInterface without touching the network stack.
func fixture(iface net.Interface, addrs ...string) RawInterface {
	return RawInterface{Interface: iface, Addrs: addrs}
}

func TestFromSystemMapsFields(t *testing.T) {
	raws := []RawInterface{
		fixture(net.Interface{
			Index:        5,
			Name:         "VPN",
			MTU:          1400,
			HardwareAddr: net.HardwareAddr{0, 0, 0, 0, 0, 0},
			Flags:        net.FlagPointToPoint,
		}),
		fixture(net.Interface{
			Index:        12,
			Name:         "Ethernet",
			MTU:          1500,
			HardwareAddr: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
		}, "192.168.1.42/24", "fe80::1234/64"),
		fixture(net.Interface{
			Index: 1,
			Name:  "Loopback",
			MTU:   65536,
			Flags: net.FlagUp | net.FlagLoopback,
		}, "127.0.0.1/8", "::1/128"),
	}

	got := FromSystem(raws)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	// Sorted by index, so loopback (index 1) comes first.
	loop := got[0]
	if loop.Name != "Loopback" || !loop.Up || !loop.Loopback {
		t.Errorf("loopback = %+v, want up loopback named Loopback", loop)
	}
	if loop.HardwareAddr != "" {
		t.Errorf("loopback MAC = %q, want empty", loop.HardwareAddr)
	}
	if len(loop.Addresses) != 2 {
		t.Fatalf("loopback addresses = %d, want 2 (IPv4 and IPv6 must both survive)", len(loop.Addresses))
	}
	if loop.Addresses[0].CIDR != "127.0.0.1/8" || loop.Addresses[1].CIDR != "::1/128" {
		t.Errorf("loopback addresses = %v, want 127.0.0.1/8 and ::1/128", loop.Addresses)
	}
	if loop.Addresses[1].IsIPv6() != true {
		t.Error("second loopback address was not detected as IPv6")
	}

	// Index 5 is the point-to-point tunnel, index 12 the Ethernet adapter.
	vpn := got[1]
	if vpn.Name != "VPN" {
		t.Fatalf("got[1].Name = %q, want VPN", vpn.Name)
	}
	if vpn.Up {
		t.Error("interface without FlagUp was reported as up")
	}
	if !vpn.PointToPoint {
		t.Error("point-to-point flag was not preserved")
	}
	if len(vpn.Addresses) != 0 {
		t.Errorf("addresses = %v, want none", vpn.Addresses)
	}
	if vpn.HardwareAddr != "" {
		t.Errorf("all-zero MAC = %q, want empty", vpn.HardwareAddr)
	}

	eth := got[2]
	if eth.Name != "Ethernet" || !eth.Up || eth.Loopback {
		t.Errorf("ethernet = %+v, want up non-loopback", eth)
	}
	if eth.MTU != 1500 {
		t.Errorf("MTU = %d, want 1500", eth.MTU)
	}
	if eth.HardwareAddr != "00:11:22:33:44:55" {
		t.Errorf("MAC = %q, want 00:11:22:33:44:55", eth.HardwareAddr)
	}
}

func TestFromSystemSkipsUnrepresentableAddresses(t *testing.T) {
	raws := []RawInterface{
		fixture(net.Interface{Index: 1, Name: "eth0", Flags: net.FlagUp},
			"192.168.1.42/24",
			"garbage",
			"",
			"fe80::1/64",
		),
	}

	got := FromSystem(raws)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(got[0].Addresses) != 2 {
		t.Fatalf("addresses = %v, want the two valid ones", got[0].Addresses)
	}
	if got[0].Addresses[0].CIDR != "192.168.1.42/24" || got[0].Addresses[1].CIDR != "fe80::1/64" {
		t.Errorf("addresses = %v, want the valid ones in order", got[0].Addresses)
	}
}

func TestFromSystemEmptyInput(t *testing.T) {
	got := FromSystem(nil)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestSortIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	// Deliberately shuffled, including an index tie to exercise the name
	// tie-breaker.
	input := []Interface{
		{Index: 12, Name: "Ethernet"},
		{Index: 1, Name: "Loopback"},
		{Index: 5, Name: "zeta"},
		{Index: 5, Name: "alpha"},
		{Index: 3, Name: "Wi-Fi"},
	}
	original := make([]Interface, len(input))
	copy(original, input)

	first := Sort(input)
	second := Sort(input)

	wantOrder := []string{"Loopback", "Wi-Fi", "alpha", "zeta", "Ethernet"}
	for i, want := range wantOrder {
		if first[i].Name != want {
			t.Errorf("sorted[%d].Name = %q, want %q (order: %v)", i, first[i].Name, want, names(first))
		}
	}
	for i := range first {
		if first[i].Name != second[i].Name {
			t.Fatalf("Sort is not deterministic: %v vs %v", names(first), names(second))
		}
	}
	for i := range input {
		if input[i].Name != original[i].Name {
			t.Errorf("Sort mutated its input: %v", names(input))
			break
		}
	}
}

func names(ifs []Interface) []string {
	out := make([]string, 0, len(ifs))
	for _, iface := range ifs {
		out = append(out, iface.Name)
	}
	return out
}

func TestListPropagatesSystemError(t *testing.T) {
	restore := stubSystem(func() ([]RawInterface, error) {
		return nil, errors.New("no interfaces here")
	})
	defer restore()

	ifs, err := List()
	if err == nil {
		t.Fatalf("List() = %+v, want error", ifs)
	}
	if want := "failed to list network interfaces"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
	if want := "no interfaces here"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to wrap the system error %q", err.Error(), want)
	}
}

func TestListUsesSystemSource(t *testing.T) {
	restore := stubSystem(func() ([]RawInterface, error) {
		return []RawInterface{
			fixture(net.Interface{Index: 2, Name: "second", Flags: net.FlagUp}, "10.0.0.1/8"),
			fixture(net.Interface{Index: 1, Name: "first", Flags: net.FlagUp}, "10.0.0.2/8"),
		}, nil
	})
	defer restore()

	ifs, err := List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(ifs) != 2 {
		t.Fatalf("len = %d, want 2", len(ifs))
	}
	if ifs[0].Name != "first" || ifs[1].Name != "second" {
		t.Errorf("order = %v, want first then second", names(ifs))
	}
}

// stubSystem replaces the package level system source for the duration of a
// test and returns a function that restores it.
func stubSystem(fn func() ([]RawInterface, error)) func() {
	prev := systemInterfaces
	systemInterfaces = fn
	return func() { systemInterfaces = prev }
}
