//go:build windows

package ports

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// Table classes and address families from the IP Helper API (iphlpapi.h).
//
// The TCP class matters: TCP_TABLE_OWNER_PID_ALL is 5, while 4 is
// TCP_TABLE_OWNER_PID_CONNECTIONS, which silently omits every LISTENING socket.
// Using the wrong one produces a listing that looks plausible but is missing
// exactly the sockets this command exists to show.
const (
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1

	afInet  = 2
	afInet6 = 23

	winFalse = 0
)

// Row sizes as the API actually packs them into the returned buffer.
//
// These are deliberately not unsafe.Sizeof of a Go mirror of the MIB_...
// structures: the API leaves out the padding a C compiler would insert.
// Every value below was verified against a live table on Windows 11 amd64
// (see source_windows_test.go, which asserts the sizes against real tables).
const (
	tcpRowSizeV4 = 24 // state, local addr, local port, remote addr, remote port, pid - six uint32
	tcpRowSizeV6 = 56 // 8-byte local scope + address, 20-byte local slot, same for remote, state, pid
	udpRowSizeV4 = 12 // local addr, local port, pid - three uint32
	udpRowSizeV6 = 28 // 8-byte scope + address, 20-byte slot, pid
)

// errInsufficientBuffer mirrors ERROR_INSUFFICIENT_BUFFER, which the IP Helper
// API returns together with the required buffer length.
var errInsufficientBuffer = syscall.Errno(122)

// syscallError wraps a non-zero Win32 return value.
type syscallError struct {
	call string
	code syscall.Errno
}

func (e *syscallError) Error() string {
	return fmt.Sprintf("%s failed: %v", e.call, error(e.code))
}

func (e *syscallError) Unwrap() error { return error(e.code) }

// syscallErrno converts a Win32 return code into an error.
func syscallErrno(code uintptr, call string) error {
	if code == 0 {
		return nil
	}
	return &syscallError{call: call, code: syscall.Errno(code)}
}

var (
	modiphlpapi             = syscall.NewLazyDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")
)

// rowKind selects one table and its row layout.
type rowKind uint8

const (
	rowKindTCP4 rowKind = iota
	rowKindTCP6
	rowKindUDP4
	rowKindUDP6
)

func (k rowKind) class() uint32 {
	if k == rowKindTCP4 || k == rowKindTCP6 {
		return tcpTableOwnerPIDAll
	}
	return udpTableOwnerPID
}

func (k rowKind) family() uint32 {
	if k == rowKindTCP4 || k == rowKindUDP4 {
		return afInet
	}
	return afInet6
}

func (k rowKind) rowSize() int {
	switch k {
	case rowKindTCP4:
		return tcpRowSizeV4
	case rowKindTCP6:
		return tcpRowSizeV6
	case rowKindUDP4:
		return udpRowSizeV4
	default:
		return udpRowSizeV6
	}
}

func (k rowKind) label() string {
	switch k {
	case rowKindTCP4:
		return "TCP/IPv4"
	case rowKindTCP6:
		return "TCP/IPv6"
	case rowKindUDP4:
		return "UDP/IPv4"
	default:
		return "UDP/IPv6"
	}
}

// platformSource enumerates sockets through the IP Helper API.
type platformSource struct{}

// Records returns the TCP and UDP sockets of the local machine.
func (platformSource) Records() ([]Record, error) {
	var records []Record

	// Each family is enumerated separately: the tables are per address family
	// and either of them may legitimately be empty.
	for _, kind := range []rowKind{rowKindTCP4, rowKindTCP6, rowKindUDP4, rowKindUDP6} {
		got, err := readTable(kind)
		if err != nil {
			return nil, err
		}
		records = append(records, got...)
	}
	return records, nil
}

// readTable enumerates one table and converts its rows.
func readTable(kind rowKind) ([]Record, error) {
	class, family := kind.class(), kind.family()

	var buf []byte
	var size uint32
	var err error

	// Size first, then read: the table can grow in between, which the API
	// reports as ERROR_INSUFFICIENT_BUFFER together with the new size.
	for attempt := 0; attempt < 3; attempt++ {
		buf = make([]byte, size)
		err = getTable(family, class, buf, &size)
		if err == nil {
			break
		}
		if !errors.Is(err, errInsufficientBuffer) {
			return nil, fmt.Errorf("list %s ports: %w", kind.label(), err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("list %s ports: %w", kind.label(), err)
	}
	if len(buf) < 4 {
		return nil, nil
	}

	count := *(*uint32)(unsafe.Pointer(&buf[0]))
	rows := buf[4:]
	base := tablePtr(rows)
	if base == nil {
		return nil, nil
	}

	rowSize := uintptr(kind.rowSize())
	total := uintptr(len(rows))

	out := make([]Record, 0, count)
	for i := uint32(0); i < count; i++ {
		offset := uintptr(i) * rowSize
		if offset+rowSize > total {
			// The buffer is shorter than the count claims; trust the buffer.
			break
		}
		// Adding a byte offset to an unsafe.Pointer is safe while the result
		// stays inside the same allocation, which the check above guarantees.
		rec, ok := kind.convert(unsafe.Add(base, offset))
		if !ok {
			// A row that cannot be represented is skipped rather than failing
			// the whole listing.
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

// getTable fills buf with the requested table, or reports that a larger buffer
// is needed.
//
// The return value, not the error, signals failure: syscall.LazyProc.Call
// reports the Win32 result code as its first value, while the error is only
// meaningful when the DLL or the function cannot be loaded.
func getTable(family, class uint32, buf []byte, size *uint32) error {
	proc := procGetExtendedTcpTable
	if class != tcpTableOwnerPIDAll {
		proc = procGetExtendedUdpTable
	}

	ret, _, _ := proc.Call(
		uintptr(tablePtr(buf)),
		uintptr(unsafe.Pointer(size)),
		winFalse,
		uintptr(family),
		uintptr(class),
		0,
	)
	if ret == 0 {
		return nil
	}
	if syscall.Errno(ret) == errInsufficientBuffer {
		return errInsufficientBuffer
	}
	return syscallErrno(ret, proc.Name)
}

// tablePtr returns the base pointer of buf, or nil when it is empty.
func tablePtr(buf []byte) unsafe.Pointer {
	if len(buf) == 0 {
		return nil
	}
	return unsafe.Pointer(&buf[0])
}
