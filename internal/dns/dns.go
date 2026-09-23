// Package dns performs forward DNS resolution: it turns a host name into the IP
// addresses a resolver reports for it.
//
// The package returns a plain domain model so that the CLI, a future TUI and
// JSON output can all consume the same values. It exposes no net.Resolver and no
// net.IPAddr to its callers: the resolver is reached through a small seam, which
// is what makes the whole package testable without a network.
//
// This is a network operation, unlike the other packages in this project, so
// Resolve takes a context.Context and honours cancellation.
package dns

import (
	"context"
	"net"
	"sort"
	"time"
)

// DefaultTimeout is the resolution budget used when the caller does not choose
// one. It lives here rather than in the resolver so that the policy is a
// property of the operation, and the CLI turns it into a context deadline.
const DefaultTimeout = 5 * time.Second

// RecordType is the type of a resolved record.
//
// The standard library resolver reports addresses, not DNS resource records, so
// only the two address families can be claimed. Anything else would be invented.
type RecordType uint8

// Record types.
const (
	// TypeA is an IPv4 address record.
	TypeA RecordType = iota + 1
	// TypeAAAA is an IPv6 address record.
	TypeAAAA
)

// String implements fmt.Stringer.
func (t RecordType) String() string {
	switch t {
	case TypeA:
		return "A"
	case TypeAAAA:
		return "AAAA"
	default:
		return "TYPE(?)"
	}
}

// Record is a single address resolved for a host.
type Record struct {
	Type    RecordType
	Address string // textual form, for example "93.184.216.34" or "2606:2800::1"
}

// Result is the outcome of one forward resolution.
type Result struct {
	// Host is the name that was looked up, as given by the caller.
	Host string
	// Records holds every address the resolver reported, ordered.
	Records []Record
	// Duration is how long the resolver call itself took. It excludes argument
	// parsing and rendering.
	Duration time.Duration
}

// Counts returns how many A and AAAA records were found.
func (r Result) Counts() (a, aaaa int) {
	for _, rec := range r.Records {
		switch rec.Type {
		case TypeA:
			a++
		case TypeAAAA:
			aaaa++
		}
	}
	return a, aaaa
}

// Empty reports whether the resolution succeeded but produced no addresses.
//
// This is not a failure: a name can legitimately resolve to nothing within the
// records the resolver asked for.
func (r Result) Empty() bool { return len(r.Records) == 0 }

// resolver is the seam between this package and the network.
//
// The signature intentionally mirrors net.Resolver.LookupIPAddr so that the
// standard-library resolver satisfies it without an adapter, while tests can
// substitute a fake. It is unexported so that neither the CLI nor callers ever
// depend on a concrete resolver type; Resolve and ResolveWith are the API.
type resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// defaultResolver is the real resolver used by Resolve.
var defaultResolver resolver = net.DefaultResolver

// Resolve looks up host using the system resolver, with the caller's context.
//
// The context is passed straight through to the resolver, so a cancelled parent
// context — for example Ctrl-C through signal.NotifyContext — aborts the lookup,
// and a deadline set by the caller bounds it. This function sets no timeout of
// its own: timeout policy belongs to the caller.
func Resolve(ctx context.Context, host string) (Result, error) {
	return ResolveWith(ctx, defaultResolver, host)
}

// ResolveWith looks up host through the supplied resolver.
//
// It is exported so that the resolution logic can be exercised without touching
// DNS. r must not be nil.
func ResolveWith(ctx context.Context, r resolver, host string) (Result, error) {
	start := time.Now()

	addrs, err := r.LookupIPAddr(ctx, host)
	duration := time.Since(start)

	if err != nil {
		// No partial Result: a failed lookup must not look like an empty one.
		return Result{}, wrapLookupError(host, err)
	}

	return Result{
		Host:     host,
		Records:  RecordsFromAddrs(addrs),
		Duration: duration,
	}, nil
}

// RecordsFromAddrs converts resolver output into the domain model, ordered.
//
// It is exported because it is the pure half of the package: the mapping and the
// ordering are exactly what tests need to pin, and they are testable without a
// resolver at all.
//
// The converter is deliberately faithful: it does not deduplicate. If a resolver
// reports the same address twice, the result contains it twice, because dropping
// data here would hide what the resolver actually answered.
func RecordsFromAddrs(addrs []net.IPAddr) []Record {
	records := make([]Record, 0, len(addrs))
	for _, addr := range addrs {
		if len(addr.IP) == 0 {
			// Neither nil nor an empty net.IP can be printed or represented;
			// skip it rather than emitting a record with an empty address. A
			// net.IP is a slice, so nil and empty both have to be rejected.
			continue
		}

		// net.IP.To4 reports the IPv4 form for both a 4-byte and a 16-byte
		// IPv4-mapped address, so this classifies mapped addresses as IPv4,
		// which is what a user expects to see.
		recordType := TypeAAAA
		ip := addr.IP
		if ip4 := ip.To4(); ip4 != nil {
			recordType = TypeA
			ip = ip4
		}

		records = append(records, Record{
			Type:    recordType,
			Address: ip.String(),
		})
	}

	return Sort(records)
}

// Sort orders records deterministically: IPv4 before IPv6, and within a family
// by address family and then lexicographically by their textual form.
//
// The resolver's own order is unspecified, so nothing may depend on it. Sorting
// here rather than in the renderer keeps the ordering a property of the data,
// which is what a future JSON output and the TUI will both need.
func Sort(records []Record) []Record {
	sorted := make([]Record, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Address < b.Address
	})
	return sorted
}
