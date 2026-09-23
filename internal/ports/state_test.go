package ports

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStateAcceptsConventionalNames(t *testing.T) {
	tests := []struct {
		in   string
		want State
	}{
		{in: "CLOSED", want: StateClosed},
		{in: "LISTENING", want: StateListen},
		{in: "ESTABLISHED", want: StateEstablished},
		{in: "SYN_SENT", want: StateSynSent},
		{in: "SYN_RECV", want: StateSynReceived},
		{in: "FIN_WAIT1", want: StateFinWait1},
		{in: "FIN_WAIT2", want: StateFinWait2},
		{in: "CLOSE_WAIT", want: StateCloseWait},
		{in: "CLOSING", want: StateClosing},
		{in: "LAST_ACK", want: StateLastAck},
		{in: "TIME_WAIT", want: StateTimeWait},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseState(tt.in)
			if err != nil {
				t.Fatalf("ParseState(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseState(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseStateIsCaseInsensitiveAndTrims(t *testing.T) {
	for _, in := range []string{"established", "Established", "eStAbLiShEd", "  ESTABLISHED  ", "\tESTABLISHED"} {
		got, err := ParseState(in)
		if err != nil {
			t.Errorf("ParseState(%q) returned error: %v", in, err)
			continue
		}
		if got != StateEstablished {
			t.Errorf("ParseState(%q) = %v, want ESTABLISHED", in, got)
		}
	}
}

func TestParseStateAcceptsAliases(t *testing.T) {
	tests := []struct {
		in   string
		want State
	}{
		{in: "LISTEN", want: StateListen},
		{in: "SYN_RECEIVED", want: StateSynReceived},
	}
	for _, tt := range tests {
		got, err := ParseState(tt.in)
		if err != nil {
			t.Errorf("ParseState(%q) returned error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseState(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseStateRejectsUnknownNames(t *testing.T) {
	// A typo must be an error, not a silently empty result.
	for _, in := range []string{"", "  ", "foobar", "ESTABLISH", "TIME-WAIT", "ESTABLISHEDX", "1"} {
		t.Run(in, func(t *testing.T) {
			if state, err := ParseState(in); err == nil {
				t.Errorf("ParseState(%q) = %v, want an error", in, state)
			}
		})
	}
}

func TestStateNamesRoundTrip(t *testing.T) {
	// Every advertised name must actually parse, so help text cannot drift from
	// the parser.
	names := StateNames()
	if len(names) == 0 {
		t.Fatal("StateNames returned nothing")
	}
	for _, name := range names {
		state, err := ParseState(name)
		if err != nil {
			t.Errorf("advertised name %q does not parse: %v", name, err)
			continue
		}
		if state.String() != name {
			t.Errorf("StateNames advertises %q but it parses to %v (String %q)",
				name, state, state.String())
		}
	}
}

func TestStateNamesMatchString(t *testing.T) {
	// The advertised list must contain exactly the states a user can see, in the
	// order they are declared, so it stays a faithful subset of the model.
	want := []string{
		"CLOSED", "LISTENING", "SYN_SENT", "SYN_RECV", "ESTABLISHED",
		"FIN_WAIT1", "FIN_WAIT2", "CLOSE_WAIT", "CLOSING", "LAST_ACK", "TIME_WAIT",
	}
	got := StateNames()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("StateNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseStateErrorMessageMentionsTheName(t *testing.T) {
	_, err := ParseState("foobar")
	if err == nil {
		t.Fatal("ParseState(\"foobar\") returned no error")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Errorf("error = %q, want it to mention the rejected name", err.Error())
	}
	// The sentinel wrapper is the CLI's job, so this must not be ErrUsage.
	if errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want a plain parse error", err)
	}
}
