package dns

import (
	"context"
	"net"
	"testing"
	"time"
)

// fakeResolver stands in for the network. Every domain test uses it, so no test
// needs DNS, a network interface or the internet.
type fakeResolver struct {
	addrs []net.IPAddr
	err   error

	// calls records every host the resolver was asked for, which lets a test
	// assert that Resolve really did (or did not) reach the resolver.
	calls []string

	// delay blocks the fake lookup until it elapses or ctx is done, so that
	// timeout and cancellation behaviour can be exercised deterministically.
	delay time.Duration
}

func (f *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	f.calls = append(f.calls, host)

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.addrs, nil
}

func ipAddr(s string) net.IPAddr {
	return net.IPAddr{IP: net.ParseIP(s)}
}

func addresses(records []Record) []string {
	out := make([]string, 0, len(records))
	for _, rec := range records {
		out = append(out, rec.Address)
	}
	return out
}

func assertRecords(t *testing.T, got []Record, want []Record) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d records %v, want %v", len(got), addresses(got), addresses(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRecordsFromAddrsClassifiesFamilies(t *testing.T) {
	tests := []struct {
		name  string
		addrs []net.IPAddr
		want  []Record
	}{
		{
			name:  "single ipv4",
			addrs: []net.IPAddr{ipAddr("93.184.216.34")},
			want:  []Record{{TypeA, "93.184.216.34"}},
		},
		{
			name:  "single ipv6",
			addrs: []net.IPAddr{ipAddr("2606:2800:220:1:248:1893:25c8:1946")},
			want:  []Record{{TypeAAAA, "2606:2800:220:1:248:1893:25c8:1946"}},
		},
		{
			name:  "loopback v4 and v6",
			addrs: []net.IPAddr{ipAddr("127.0.0.1"), ipAddr("::1")},
			want:  []Record{{TypeA, "127.0.0.1"}, {TypeAAAA, "::1"}},
		},
		{
			name: "mixed families are ordered v4 first",
			addrs: []net.IPAddr{
				ipAddr("2606:2800:220:1:248:1893:25c8:1946"),
				ipAddr("93.184.216.34"),
				ipAddr("fe80::1"),
				ipAddr("10.0.0.1"),
			},
			want: []Record{
				{TypeA, "10.0.0.1"},
				{TypeA, "93.184.216.34"},
				{TypeAAAA, "2606:2800:220:1:248:1893:25c8:1946"},
				{TypeAAAA, "fe80::1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRecords(t, RecordsFromAddrs(tt.addrs), tt.want)
		})
	}
}

// TestRecordsFromAddrsTreatsMappedIPv4AsIPv4 pins the classification of a
// 16-byte IPv4-mapped address such as "::ffff:192.168.1.1", which arrives as an
// IPv6-shaped value but is an IPv4 address to a user.
func TestRecordsFromAddrsTreatsMappedIPv4AsIPv4(t *testing.T) {
	got := RecordsFromAddrs([]net.IPAddr{ipAddr("::ffff:192.168.1.1")})
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if got[0].Type != TypeA {
		t.Errorf("Type = %v, want A", got[0].Type)
	}
	if got[0].Address != "192.168.1.1" {
		t.Errorf("Address = %q, want 192.168.1.1", got[0].Address)
	}
}

func TestRecordsFromAddrsSkipsNilIPs(t *testing.T) {
	// A nil IP cannot be printed or represented; it is dropped rather than
	// producing a record with an empty address.
	got := RecordsFromAddrs([]net.IPAddr{
		{IP: nil},
		ipAddr("192.168.1.1"),
		{IP: net.IP{}},
	})

	assertRecords(t, got, []Record{{TypeA, "192.168.1.1"}})
}

func TestRecordsFromAddrsDoesNotDeduplicate(t *testing.T) {
	// Faithfulness is deliberate: if a resolver answers with the same address
	// twice, the result says so.
	got := RecordsFromAddrs([]net.IPAddr{
		ipAddr("1.2.3.4"),
		ipAddr("1.2.3.4"),
	})

	assertRecords(t, got, []Record{{TypeA, "1.2.3.4"}, {TypeA, "1.2.3.4"}})
}

func TestRecordsFromAddrsEmptyAndNil(t *testing.T) {
	if got := RecordsFromAddrs(nil); len(got) != 0 {
		t.Errorf("RecordsFromAddrs(nil) = %v, want empty", got)
	}
	if got := RecordsFromAddrs([]net.IPAddr{}); len(got) != 0 {
		t.Errorf("RecordsFromAddrs(empty) = %v, want empty", got)
	}
}

func TestRecordsFromAddrsDoesNotMutateInput(t *testing.T) {
	addrs := []net.IPAddr{ipAddr("2606:2800::1"), ipAddr("10.0.0.1")}
	before := make([]net.IPAddr, len(addrs))
	copy(before, addrs)

	_ = RecordsFromAddrs(addrs)

	for i := range addrs {
		if !addrs[i].IP.Equal(before[i].IP) {
			t.Fatalf("input was mutated at %d: %v, want %v", i, addrs[i].IP, before[i].IP)
		}
	}
}
