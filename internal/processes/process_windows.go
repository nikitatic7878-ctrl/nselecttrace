//go:build windows

package processes

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// winProcessQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION, the
// least privilege that still allows QueryFullProcessImageName to succeed. Using
// it instead of PROCESS_QUERY_INFORMATION lets nselecttrace read the image name of
// as many processes as the current user is allowed to see, without requiring
// administrator rights.
const winProcessQueryLimitedInformation = 0x1000

var (
	modkernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess               = modkernel32.NewProc("OpenProcess")
	procCloseHandle               = modkernel32.NewProc("CloseHandle")
	procQueryFullProcessImageName = modkernel32.NewProc("QueryFullProcessImageNameW")
)

// unreadableName is returned for PIDs that exist but cannot be inspected: PID 0
// (the idle process on Windows) and processes owned by another user. It is not
// an error, because the caller can still report the socket.
const unreadableName = ""

// systemProcessName resolves a PID to its executable name on Windows.
func systemProcessName(pid uint32) (string, error) {
	if pid == 0 {
		// PID 0 is the idle process; it has no image and cannot be opened.
		return unreadableName, nil
	}

	handle, err := openProcess(pid)
	if err != nil {
		// The process exists but we are not allowed to look at it: report the
		// socket without a name rather than failing the whole listing.
		return unreadableName, nil
	}
	defer closeHandle(handle)

	imagePath, err := queryImageName(handle)
	if err != nil {
		return unreadableName, nil
	}
	return baseName(imagePath), nil
}

// openProcess opens a process handle with query-only rights.
func openProcess(pid uint32) (syscall.Handle, error) {
	handle, _, err := procOpenProcess.Call(
		winProcessQueryLimitedInformation,
		0, // not inheritable
		uintptr(pid),
	)
	if handle == 0 {
		if err == syscall.Errno(0) {
			err = errors.New("OpenProcess failed")
		}
		return 0, fmt.Errorf("open process %d: %w", pid, err)
	}
	return syscall.Handle(handle), nil
}

// closeHandle releases a handle. The result is intentionally ignored: failing
// to close a handle is not something the caller can act on, and the process is
// about to exit anyway.
func closeHandle(handle syscall.Handle) {
	_, _, _ = procCloseHandle.Call(uintptr(handle))
}

// queryImageName reads the executable path of an opened process.
func queryImageName(handle syscall.Handle) (string, error) {
	// MAX_PATH is not enough for long paths; 1024 UTF-16 code units is the
	// size used by the Windows API samples.
	buf := make([]uint16, 1024)
	size := uint32(len(buf))

	ret, _, err := procQueryFullProcessImageName.Call(
		uintptr(handle),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		if err == syscall.Errno(0) {
			err = errors.New("QueryFullProcessImageName failed")
		}
		return "", fmt.Errorf("query image name: %w", err)
	}
	return syscall.UTF16ToString(buf[:size]), nil
}

// baseName reduces a full Windows path to its file name component.
//
// It is implemented here instead of with path/filepath because nselecttrace also
// runs on Unix-like systems, where a Windows path must not be interpreted using
// forward slashes.
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
