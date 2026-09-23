package cli

import (
	"strconv"
	"strings"

	"nselecttrace/internal/ports"
)

// portsFlags accumulates the flags shared by `nselecttrace ports` and
// `nselecttrace connections`.
//
// Both commands accept the same spelling for the same concept, so the parsing
// lives here rather than being duplicated: a flag that works in one command
// works in the other by construction, not by copied code.
type portsFlags struct {
	command  string // command name, used in error messages
	filter   ports.Filter
	wantTCP  bool
	wantUDP  bool
	stateSet bool
}

// parsePortsFlags converts args into a ports.Filter.
//
// The caller passes the set of extra flags it understands; anything else is
// rejected with the usage error model. Values may be written either as
// "--flag value" or "--flag=value".
func parsePortsFlags(command string, args []string, extra func(name string, takeValue func() (string, error)) (bool, error)) (ports.Filter, error) {
	flags := portsFlags{command: command}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := strings.Cut(arg, "=")

		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", UsageError("%s: %s requires a value", command, name)
			}
			return args[i], nil
		}

		if extra != nil {
			handled, err := extra(name, takeValue)
			if err != nil {
				return flags.filter, err
			}
			if handled {
				continue
			}
		}

		switch name {
		case "--tcp":
			flags.wantTCP = true
		case "--udp":
			flags.wantUDP = true
		case "--port":
			raw, err := takeValue()
			if err != nil {
				return flags.filter, err
			}
			port, err := parsePortNumber(command, raw)
			if err != nil {
				return flags.filter, err
			}
			flags.filter.LocalPort = port
		case "--pid":
			raw, err := takeValue()
			if err != nil {
				return flags.filter, err
			}
			pid, err := strconv.ParseUint(raw, 10, 32)
			if err != nil {
				return flags.filter, UsageError("%s: invalid PID %q", command, raw)
			}
			flags.filter.PID = uint32(pid)
		case "--process":
			raw, err := takeValue()
			if err != nil {
				return flags.filter, err
			}
			if raw == "" {
				return flags.filter, UsageError("%s: --process requires a value", command)
			}
			flags.filter.ProcessSub = raw
		case "--state":
			raw, err := takeValue()
			if err != nil {
				return flags.filter, err
			}
			state, err := ports.ParseState(raw)
			if err != nil {
				return flags.filter, UsageError("%s: invalid state %q (want one of %s)",
					command, raw, strings.Join(ports.StateNames(), ", "))
			}
			flags.filter.State = state
			flags.stateSet = true
		default:
			return flags.filter, UsageError("%s: unknown argument %q", command, arg)
		}
	}

	switch {
	case flags.wantTCP && flags.wantUDP:
		// Asking for both is the same as asking for either; leaving Protocol at
		// its zero value keeps the filter meaningful.
		flags.filter.Protocol = 0
	case flags.wantTCP:
		flags.filter.Protocol = ports.TCP
	case flags.wantUDP:
		flags.filter.Protocol = ports.UDP
	}

	return flags.filter, nil
}

// parsePortNumber validates a TCP/UDP port number.
func parsePortNumber(command, raw string) (uint16, error) {
	n, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || n == 0 {
		return 0, UsageError("%s: invalid port %q (want 1-65535)", command, raw)
	}
	return uint16(n), nil
}
