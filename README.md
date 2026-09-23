# nselecttrace

[English](README.md) | [Русский](README.ru.md)

`nselecttrace` is a small cross-platform CLI for network inspection and diagnostics.

It answers the everyday questions you ask about a machine's network — which
interfaces exist, which ports are listening, who owns them, what is currently
connected — from a single binary with no runtime dependencies and no shelling
out to `netstat`, `ss`, `nslookup` or `ping`.

This is an early-stage project. It is not a packet sniffer, not a network
scanner and not a monitoring server. It is a read-only diagnostic tool: it
reports what the operating system already knows, and for `dns` and `ping` it
performs one clearly-scoped network operation.

## What is nselecttrace?

- **Local inspection first.** `interfaces`, `ports` and `connections` read the
  host's own state and never probe the network.
- **Two explicit network operations.** `dns` performs a forward lookup; `ping`
  sends ICMP echo requests. Nothing else touches the network.
- **Process ownership.** Socket listings resolve the owning process, so a
  listening port is attributed to a program and not only to a number.
- **Native APIs where they matter.** Windows socket enumeration uses the IP
  Helper API (`GetExtendedTcpTable` / `GetExtendedUdpTable`) instead of parsing
  the output of a system command.
- **Standard library only.** There are no third-party dependencies, which keeps
  the binary small and the build reproducible. ICMP is implemented directly on
  Go's `net` package rather than adding a networking library.
- **Machine-friendly output.** Results go to stdout, warnings and errors to
  stderr, so piping a table into another tool stays clean.

## Features

| Command | What it does |
|---|---|
| `interfaces` | Network interfaces with state, MAC and IPv4/IPv6 addresses |
| `ports` | Listening and bound TCP/UDP sockets with their owning process |
| `connections` | Active connections, showing local and remote endpoints |
| `dns <host>` | Forward DNS resolution to A and AAAA records |
| `ping <host>` | ICMP echo reachability with per-packet round-trip time |
| `about`, `version` | Host, runtime and build information |

Listings are sorted deterministically, so repeated runs produce identical
output. Filters are applied to the resolved data model, not pushed down into the
platform API, so a filter means the same thing on every platform.

## Commands

Run `nselecttrace --help` for the command list, or
`nselecttrace <command> --help` for the flags of one command.

### interfaces

Lists every network interface in ascending interface-index order: name, up/down
state, MAC address and all assigned addresses with their prefix length. IPv4 and
IPv6 are both reported; a missing value shows as `-`.

```console
$ nselecttrace interfaces
NAME              STATE  MAC                ADDRESSES
Loopback          UP     -                  127.0.0.1/8
                                         ::1/128
Ethernet          UP     00:11:22:33:44:55  192.168.1.42/24
                                         fe80::1234:5678:9abc:def0/64
Wi-Fi             DOWN   66:77:88:99:aa:bb  -
```

`--verbose` adds the interface index, MTU and whether the interface is loopback
or point-to-point:

```console
$ nselecttrace interfaces --verbose
Ethernet  (regular)
  index:      12
  state:      UP
  mac:        00:11:22:33:44:55
  mtu:        1500
  addresses:  192.168.1.42/24
              fe80::1234:5678:9abc:def0/64
```

### ports

Lists the bound and listening sockets of the local machine: protocol, local
address, TCP state, owning PID and process name. UDP has no state machine, so a
UDP socket shows `-` in the STATE column rather than a fabricated `LISTENING`.

```console
$ nselecttrace ports
PROTO  LOCAL ADDRESS      STATE        PID    PROCESS
TCP    0.0.0.0:135        LISTENING    1200   svchost.exe
TCP    0.0.0.0:445        LISTENING    4      System
TCP    127.0.0.1:3000     LISTENING    8212   node.exe
TCP    192.168.1.6:5432   ESTABLISHED  9144   postgres.exe
UDP    0.0.0.0:53         -            1400   <unknown>
```

```bash
nselecttrace ports                      # every bound and listening socket
nselecttrace ports --tcp                # TCP only
nselecttrace ports --udp                # UDP only
nselecttrace ports --port 8080          # only this local port
nselecttrace ports --pid 1234           # only sockets of this process
nselecttrace ports --process node       # by process name, case-insensitive
nselecttrace ports --state LISTENING    # only this TCP state
```

### connections

Lists the **active** connections: the sockets that have a remote endpoint. This
is the difference from `ports`, which lists bound and listening sockets.

```console
$ nselecttrace connections
PROTO  LOCAL              REMOTE                 STATE        PID    PROCESS
TCP    127.0.0.1:52143    127.0.0.1:3000         ESTABLISHED  8212   node.exe
TCP    192.168.1.6:52341  142.250.74.14:443      ESTABLISHED  8416   chrome.exe
TCP    192.168.1.6:53122  192.168.1.10:22        TIME_WAIT    0      <unknown>
TCP    [fe80::1234]:53126 [2607:f8b0::1]:443     ESTABLISHED  8416   chrome.exe
```

Remote addresses stay addresses: `connections` never resolves them to host names
and never sends a packet. IPv6 endpoints are bracketed so the colons inside the
address cannot be mistaken for the port separator.

```bash
nselecttrace connections                     # every active connection
nselecttrace connections --state ESTABLISHED # one TCP state
nselecttrace connections --remote-port 443   # everything talking to 443
nselecttrace connections --tcp               # TCP only
nselecttrace connections --udp               # UDP connections that have a peer
nselecttrace connections --process chrome    # by process name
nselecttrace connections --pid 8416          # by owning process ID
```

`--state` accepts the TCP states that can actually be observed: `CLOSED`,
`LISTENING`, `SYN_SENT`, `SYN_RECV`, `ESTABLISHED`, `FIN_WAIT1`, `FIN_WAIT2`,
`CLOSE_WAIT`, `CLOSING`, `LAST_ACK`, `TIME_WAIT` (case-insensitive). An unknown
state is an error, not an empty result.

### dns

Forward DNS resolution: it turns a host name into the addresses the system
resolver reports and labels each one by family.

```console
$ nselecttrace dns example.com
HOST         TYPE  ADDRESS
example.com  A     93.184.216.34
example.com  AAAA  2606:2800:220:1:248:1893:25c8:1946

Resolved  : 2 records (1 A, 1 AAAA)
Duration  : 24.7ms
```

- **A and AAAA only.** Those are the records the Go resolver reports. MX, TXT,
  CNAME, SRV and the rest are not queried, and are not claimed.
- **Forward resolution only.** There is no reverse (PTR) lookup. An IP address
  given as the host is passed to the resolver like any other input.
- **`--timeout`** bounds the lookup and defaults to `5s`.
- **A failed lookup is a failure.** A resolver error is reported as an error and
  exits non-zero; it is never shown as a successful empty result. A lookup that
  succeeds but returns nothing prints `No records found` and exits `0`.
- `Duration` is the time the resolver call itself took, excluding argument
  parsing and rendering.

```bash
nselecttrace dns example.com
nselecttrace dns example.com --timeout 5s
nselecttrace dns example.com --timeout=500ms
```

```console
$ nselecttrace dns example.invalid
nselecttrace: failed to resolve "example.invalid": lookup example.invalid: no such host
```

### ping

Sends ICMP echo requests and reports the round-trip time of each reply.

```console
$ nselecttrace ping localhost --count 2
HOST       ADDRESS    SEQ  STATUS  TIME
localhost  127.0.0.1  1    OK      0µs
localhost  127.0.0.1  2    OK      0µs

Sent     : 2
Received : 2
Lost     : 0
```

```bash
nselecttrace ping localhost                 # four echo requests
nselecttrace ping 1.1.1.1 --count 3         # three echo requests
nselecttrace ping localhost --timeout 5s    # bound the whole run
nselecttrace ping example.com --timeout=500ms
```

- **`--count`** defaults to `4`. A value of `0` or less is a usage error; there
  is no continuous mode.
- **`--timeout`** defaults to `5s` and bounds the whole run. An individual
  request that goes unanswered within a shorter per-packet budget produces a
  `TIMEOUT` row and the run continues, so partial loss is reported rather than
  aborting everything.
- **One address per run.** A host name may resolve to several addresses; exactly
  one is probed, chosen deterministically (IPv4 first, then IPv6, lowest address
  within a family). The chosen address appears in the ADDRESS column.
- **Packet loss is not a failure.** A completed run with lost packets is a normal
  result and exits `0`. A timed-out row shows `-` for TIME: no round-trip time is
  invented.

```console
$ nselecttrace ping 192.0.2.1 --count 2
HOST       ADDRESS    SEQ  STATUS   TIME
192.0.2.1  192.0.2.1  1    TIMEOUT  -
192.0.2.1  192.0.2.1  2    TIMEOUT  -

Sent     : 2
Received : 0
Lost     : 2
```

- **ICMP availability depends on the environment.** Windows and macOS normally
  allow ICMP for a regular user. Linux generally requires root or a
  `ping_group_range` that covers the user. When the socket cannot be opened the
  error says so and stdout stays empty, rather than reporting a fabricated
  result.
- Ctrl-C cancels a run immediately, and a cancelled run is reported as a
  cancellation rather than as packet loss.

### about / version

```console
$ nselecttrace about
nselecttrace
Network diagnostic utility

host:    MY-HOST
runtime: windows/amd64
version: 0.1.0

$ nselecttrace version
nselecttrace 0.1.0
```

`version` reports the release version, the Git commit and the build time. Those
are stamped in at build time through `-ldflags` (see `internal/buildinfo` and the
`Makefile`), so a binary built straight from source shows `unknown` for the
commit and the build date.

## Examples

```bash
# Which interfaces does this machine have, and what addresses?
nselecttrace interfaces

# What is listening, and which process owns it?
nselecttrace ports --state LISTENING

# Is anything talking to port 443 right now?
nselecttrace connections --remote-port 443

# Does DNS resolve this name, and how fast?
nselecttrace dns example.com

# Is this host reachable over ICMP, and with what latency?
nselecttrace ping 1.1.1.1 --count 3

# Everything a single process owns
nselecttrace ports --process node
```

## Exit codes

The same convention applies to every command.

| Code | Meaning |
|---|---|
| `0` | Success, including an empty result and a ping run with packet loss |
| `1` | Runtime failure: the operation could not be performed |
| `2` | Usage error: the command line itself was invalid |

Exit `2` covers a missing host, an extra argument, an unknown flag, an invalid
port, an invalid count, an invalid state and an invalid `--timeout`. A `2` always
prints usage to stderr and never attempts the operation. Exit `1` covers a
resolver failure, an ICMP socket that cannot be opened, an unsupported platform
backend, and a timeout or cancellation. Error messages carry the
`nselecttrace:` prefix exactly once.

## Platform support

| Command | Windows | Linux | macOS |
|---|---|---|---|
| `interfaces` | ✅ | ✅ | ✅ |
| `ports` | ✅ | ❌ not implemented | ❌ not implemented |
| `connections` | ✅ | ❌ not implemented | ❌ not implemented |
| `dns` | ✅ | ✅ | ✅ |
| `ping` | ✅ | ✅ | ✅ |

`ports` and `connections` rely on a platform-specific socket enumeration backend.
On Windows that backend uses the IP Helper API. On a platform where the backend
is not implemented, those commands report
`nselecttrace: ports are not implemented on this platform` and exit `1`; they do
not print an empty table that could be mistaken for "nothing is listening".

`connections` has no backend of its own: it reads the same socket data through
the `ports` layer, so it becomes available on a platform as soon as `ports` does,
and the two commands cannot disagree about a socket.

Every command builds for `windows`, `linux` and `darwin` on both `amd64` and
`arm64`. For `ping`, being able to build is separate from being allowed to open
an ICMP socket, which depends on the privileges and network configuration
described above.

## Installation

### From source

Requires Go 1.23 or newer.

```bash
git clone <repository-url>
cd nselecttrace
go build -o nselecttrace ./cmd/nselecttrace
```

The repository URL is shown by `git remote -v`. There is no tagged release and
no package-manager formula yet, so installation is from source.

### With the Go toolchain

Once the module is published at its final import path:

```bash
go install nselecttrace/cmd/nselecttrace@latest
```

### Windows

Build the same way, and the result is `nselecttrace.exe`:

```powershell
go build -o nselecttrace.exe ./cmd/nselecttrace
.\nselecttrace.exe ping localhost
```

### Linux and macOS

```bash
go build -o nselecttrace ./cmd/nselecttrace
./nselecttrace interfaces
```

On Linux, `ports` and `connections` are not implemented yet, and `ping` may
require elevated privileges.

## Development

```bash
go build ./...        # build everything
go run ./cmd/nselecttrace
go test ./...         # run the test suite
go vet ./...          # static checks
gofmt -l .            # formatting check (should print nothing)
```

A `Makefile` wraps the common tasks, including a build that stamps in the
version, commit and build date, and a cross-compilation target:

```bash
make build        # ./nselecttrace with version/commit/date stamped in
make test
make vet
make fmt-check
make cross        # cross-compile for windows/linux/darwin
make release      # the six release targets, version metadata stamped in
```

### Releasing

Releases are cut by pushing a version tag. The workflow in
`.github/workflows/release.yml` runs only on a `v*` tag, builds the six
target/architecture combinations, stamps the version, commit and build date in
through `-ldflags`, and attaches the archives to the GitHub release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

Archive names follow
`nselecttrace-<tag>-<os>-<arch>.<zip|tar.gz>`, for example
`nselecttrace-v0.1.0-windows-amd64.zip`. The version reported by
`nselecttrace version` comes from the tag with the leading `v` removed.

### Project layout

```text
nselecttrace/
├── cmd/
│   └── nselecttrace/     # package main: the only executable entry point
├── internal/
│   ├── buildinfo/        # version, commit, build date (overridable via -ldflags)
│   ├── cli/              # argument routing, command registry, help, exit codes
│   ├── interfaces/       # network interfaces
│   ├── ports/            # TCP/UDP sockets (the only socket data source)
│   ├── connections/      # connections = sockets with a remote endpoint
│   ├── processes/        # pid -> process name
│   ├── dns/              # forward DNS resolution
│   └── ping/             # ICMP echo reachability
├── go.mod
├── LICENSE
├── README.md
├── README.ru.md
└── .github/
    └── workflows/        # release workflow (runs on a v* tag)
```

### Design rules

- **Data collection never lives in the UI.** Every `internal/*` package returns
  plain Go structs. The CLI, a future TUI and JSON output all consume the same
  values:

  ```text
  system APIs / network  ->  internal/*  ->  cli
  ```

  A command never calls a system or network API directly.
- **Commands are additive.** A command is a value in the `cli.Command` registry.
  Adding one does not require touching `main.go`.
- **Platform differences are explicit.** Where the OS matters, build-tagged files
  sit behind a single platform-neutral API. Windows internals never leak into the
  CLI, and the CLI has no `GOOS` checks.
- **Failures are classified, not guessed.** Timeouts and cancellations are
  detected with `errors.Is`/`errors.As` against context sentinels, never by
  matching error strings.
- **Unsupported is a stated error.** A platform without a backend reports it
  plainly; no command fabricates data to look complete.

## Project status

`nselecttrace` is an early-stage open-source project.

The current `v0.1.0` release focuses on:

- network interface inspection
- local ports
- active connections
- DNS resolution
- ICMP ping

The implementation is complete and tested for the commands listed above: every
command has unit tests, the platform boundary is covered by fixture-based tests,
and the whole project builds and vets cleanly for Windows, Linux and macOS on
`amd64` and `arm64`. Commands for platforms without a backend fail honestly
rather than degrading silently.

## Roadmap

Future releases may expand network diagnostics and troubleshooting capabilities.
Candidate directions, in no particular order and not yet committed:

- a Linux and macOS socket backend, so `ports` and `connections` work beyond
  Windows
- additional ping statistics such as min/avg/max round-trip time
- TCP reachability checks and path tracing
- richer DNS output, for example querying record types beyond A/AAAA

Nothing in this list is a promise, and no interface is reserved for it.

## License

[MIT](LICENSE)

Copyright (c) 2026 nselecttrace contributors.
