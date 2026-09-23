// Package buildinfo exposes compile-time metadata about the nselecttrace binary.
package buildinfo

// Values are intended to be overridden at build time, for example:
//
//	go build -ldflags "-X nselecttrace/internal/buildinfo.Version=1.0.0"
var (
	// Version is the released version of nselecttrace.
	Version = "0.1.0"
	// Commit is the Git revision the binary was built from.
	Commit = "unknown"
	// Date is the build timestamp in RFC 3339 format.
	Date = "unknown"
)
