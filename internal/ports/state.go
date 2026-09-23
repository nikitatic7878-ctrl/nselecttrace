package ports

import (
	"fmt"
	"strings"
)

// stateNames maps the accepted command-line spellings onto the states the model
// can actually produce. It is the inverse of State.String for every value that
// is meaningful to filter on.
//
// StateNone, StateUnknown and StateDeleteTCB are intentionally absent: the first
// two are not real TCP states a user can usefully ask for, and the last is a
// transient internal state that only appears while a socket is being torn down.
var stateNames = map[string]State{
	"CLOSED":       StateClosed,
	"LISTENING":    StateListen,
	"LISTEN":       StateListen,
	"SYN_SENT":     StateSynSent,
	"SYN_RECV":     StateSynReceived,
	"SYN_RECEIVED": StateSynReceived,
	"ESTABLISHED":  StateEstablished,
	"FIN_WAIT1":    StateFinWait1,
	"FIN_WAIT2":    StateFinWait2,
	"CLOSE_WAIT":   StateCloseWait,
	"CLOSING":      StateClosing,
	"LAST_ACK":     StateLastAck,
	"TIME_WAIT":    StateTimeWait,
}

// ParseState converts a state name into a State. Matching is case-insensitive
// and accepts both the conventional spelling and the shorter alias.
//
// An unrecognised name is an error rather than a silently empty result: a typo
// in --state should not look like "no matching connections".
func ParseState(name string) (State, error) {
	state, ok := stateNames[strings.ToUpper(strings.TrimSpace(name))]
	if !ok {
		return StateNone, fmt.Errorf("unknown state %q", name)
	}
	return state, nil
}

// StateNames returns the accepted state names in a stable order, for help text.
func StateNames() []string {
	return []string{
		"CLOSED", "LISTENING", "SYN_SENT", "SYN_RECV", "ESTABLISHED",
		"FIN_WAIT1", "FIN_WAIT2", "CLOSE_WAIT", "CLOSING", "LAST_ACK", "TIME_WAIT",
	}
}
