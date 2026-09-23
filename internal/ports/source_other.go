//go:build !windows

package ports

// platformSource reports that socket enumeration is not available here.
//
// No fake data is produced: an honest error is better than a plausible looking
// listing that is not real. Linux would read /proc/net/{tcp,tcp6,udp,udp6};
// macOS would use sysctl/netstat-compatible APIs. Both are separate work.
type platformSource struct{}

// Records always fails on platforms without an implementation.
func (platformSource) Records() ([]Record, error) {
	return nil, ErrUnsupported
}
