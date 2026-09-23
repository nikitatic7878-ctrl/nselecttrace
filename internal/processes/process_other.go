//go:build !windows

package processes

// systemProcessName is not implemented outside Windows yet.
//
// Linux could read /proc/<pid>/comm or /proc/<pid>/stat, which needs no extra
// privileges; macOS would need libproc or sysctl. Rather than guess at a
// half-working implementation, the message says so plainly, and ports.List
// degrades to listing sockets without process names.
func systemProcessName(pid uint32) (string, error) {
	return "", ErrUnsupported
}
