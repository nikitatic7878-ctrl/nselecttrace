// Package processes resolves a process ID to information about the process.
//
// The package returns a small platform-neutral model so that the ports,
// connections and any future command can share it. It never shells out to
// external tools and never exposes operating system handles to callers.
package processes

import "fmt"

// Process describes a single process of the local machine.
type Process struct {
	PID  uint32
	Name string // executable name, for example "node.exe"; empty when unreadable
}

// Get returns information about the process with the given PID.
//
// A process that exists but cannot be inspected yields a Process with an empty
// Name and no error: that is an expected outcome on Windows and macOS when the
// caller lacks the required privileges. An error is returned only when the
// lookup itself failed.
func Get(pid uint32) (Process, error) {
	name, err := systemProcessName(pid)
	if err != nil {
		return Process{PID: pid}, fmt.Errorf("look up process %d: %w", pid, err)
	}
	return Process{PID: pid, Name: name}, nil
}

// ErrUnsupported reports that process lookup is not implemented on the current
// platform. It is exported so callers can decide whether to degrade or fail.
var ErrUnsupported = fmt.Errorf("process lookup is not implemented on this platform")

// Cache memoises process lookups for the lifetime of a single command run.
//
// Looking up a process is comparatively expensive, while a machine typically
// has several sockets per process. A Cache is not safe for concurrent use and
// is deliberately not shared between runs: nselecttrace keeps no persistent state.
type Cache struct {
	entries map[uint32]Process
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{entries: map[uint32]Process{}}
}

// Lookup returns the process for pid, consulting the backing store only once
// per PID.
//
// PIDs may be reused by the operating system, so a Cache must not outlive a
// single enumeration; within one run the mapping is stable enough to be worth
// caching.
func (c *Cache) Lookup(pid uint32) Process {
	if proc, ok := c.entries[pid]; ok {
		return proc
	}

	proc, err := Get(pid)
	if err != nil {
		proc = Process{PID: pid}
	}
	c.entries[pid] = proc
	return proc
}

// Len reports how many PIDs have been resolved by the cache.
func (c *Cache) Len() int { return len(c.entries) }
