package ports

import "testing"

func TestProtocolString(t *testing.T) {
	tests := []struct {
		proto Protocol
		want  string
	}{
		{TCP, "TCP"},
		{UDP, "UDP"},
		{Protocol(0), "PROTO(0)"},
		{Protocol(99), "PROTO(99)"},
	}
	for _, tt := range tests {
		if got := tt.proto.String(); got != tt.want {
			t.Errorf("Protocol(%d).String() = %q, want %q", tt.proto, got, tt.want)
		}
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNone, "-"},
		{StateClosed, "CLOSED"},
		{StateListen, "LISTENING"},
		{StateSynSent, "SYN_SENT"},
		{StateSynReceived, "SYN_RECV"},
		{StateEstablished, "ESTABLISHED"},
		{StateFinWait1, "FIN_WAIT1"},
		{StateFinWait2, "FIN_WAIT2"},
		{StateCloseWait, "CLOSE_WAIT"},
		{StateClosing, "CLOSING"},
		{StateLastAck, "LAST_ACK"},
		{StateTimeWait, "TIME_WAIT"},
		{StateDeleteTCB, "DELETE_TCB"},
		{StateUnknown, "UNKNOWN"},
		{State(200), "STATE(200)"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestUDPHasNoState(t *testing.T) {
	// The zero value must render as "no state" rather than as a TCP state, so
	// that UDP rows cannot be mistaken for LISTENING.
	var s State
	if s != StateNone {
		t.Errorf("zero State = %v, want StateNone", s)
	}
	if s.String() != "-" {
		t.Errorf("zero State renders as %q, want %q", s.String(), "-")
	}
}

func TestLocalEndpoint(t *testing.T) {
	tests := []struct {
		name string
		port Port
		want string
	}{
		{name: "ipv4 wildcard", port: Port{LocalIP: "0.0.0.0", LocalPort: 80}, want: ":80"},
		{name: "ipv6 wildcard", port: Port{LocalIP: "::", LocalPort: 443}, want: ":443"},
		{name: "empty address", port: Port{LocalIP: "", LocalPort: 53}, want: ":53"},
		{name: "specific ipv4", port: Port{LocalIP: "127.0.0.1", LocalPort: 3000}, want: "127.0.0.1:3000"},
		// IPv6 must be bracketed so the colons in the address cannot be read as
		// the port separator.
		{name: "specific ipv6 loopback", port: Port{LocalIP: "::1", LocalPort: 5353}, want: "[::1]:5353"},
		{name: "specific ipv6 link local", port: Port{LocalIP: "fe80::1234", LocalPort: 443}, want: "[fe80::1234]:443"},
		{name: "specific ipv6 global", port: Port{LocalIP: "2607:f8b0::1", LocalPort: 443}, want: "[2607:f8b0::1]:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.LocalEndpoint(); got != tt.want {
				t.Errorf("LocalEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoteEndpointFormatting(t *testing.T) {
	tests := []struct {
		name string
		port Port
		want string
	}{
		{name: "no remote", port: Port{LocalIP: "0.0.0.0", LocalPort: 80}, want: "-"},
		{name: "remote zero port", port: Port{RemoteIP: "1.1.1.1", RemotePort: 0}, want: "-"},
		{name: "ipv4", port: Port{RemoteIP: "142.250.74.14", RemotePort: 443}, want: "142.250.74.14:443"},
		{name: "ipv6 loopback", port: Port{RemoteIP: "::1", RemotePort: 8080}, want: "[::1]:8080"},
		{name: "ipv6 link local", port: Port{RemoteIP: "fe80::1234", RemotePort: 443}, want: "[fe80::1234]:443"},
		{name: "ipv6 global", port: Port{RemoteIP: "2607:f8b0::1", RemotePort: 443}, want: "[2607:f8b0::1]:443"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.port.RemoteEndpoint(); got != tt.want {
				t.Errorf("RemoteEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsWildcardAndHasRemote(t *testing.T) {
	wild := Port{LocalIP: "0.0.0.0", LocalPort: 80}
	if !wild.IsWildcard() {
		t.Error(`IsWildcard() = false for 0.0.0.0`)
	}
	if wild.HasRemote() {
		t.Error("HasRemote() = true for a listening socket")
	}

	bound := Port{LocalIP: "192.168.1.6", LocalPort: 5432}
	if bound.IsWildcard() {
		t.Error("IsWildcard() = true for a specific address")
	}
}

func TestRemoteEndpoint(t *testing.T) {
	listening := Port{LocalIP: "0.0.0.0", LocalPort: 80}
	if got := listening.RemoteEndpoint(); got != "-" {
		t.Errorf("RemoteEndpoint() = %q, want %q for a listener", got, "-")
	}

	established := Port{RemoteIP: "1.1.1.1", RemotePort: 443}
	if got := established.RemoteEndpoint(); got != "1.1.1.1:443" {
		t.Errorf("RemoteEndpoint() = %q, want %q", got, "1.1.1.1:443")
	}
}
