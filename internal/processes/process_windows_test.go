//go:build windows

package processes

import "testing"

func TestBaseName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: `C:\Windows\System32\svchost.exe`, want: "svchost.exe"},
		{path: `C:\Program Files\nselecttrace\nselecttrace.exe`, want: "nselecttrace.exe"},
		{path: `svchost.exe`, want: "svchost.exe"},
		{path: `C:/Windows/System32/drivers/etc/hosts`, want: "hosts"},
		{path: `C:\`, want: ""},
		{path: ``, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := baseName(tt.path); got != tt.want {
				t.Errorf("baseName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestGetCurrentProcess is a smoke test that does not depend on which processes
// happen to run on the machine: the test binary itself is guaranteed to exist.
func TestGetCurrentProcess(t *testing.T) {
	self := currentPID()

	proc, err := Get(self)
	if err != nil {
		t.Fatalf("Get(%d) returned error: %v", self, err)
	}
	if proc.PID != self {
		t.Errorf("PID = %d, want %d", proc.PID, self)
	}
	if proc.Name == "" {
		t.Error("Name is empty for the current process, want the test binary name")
	}
	t.Logf("current process: %d (%s)", proc.PID, proc.Name)
}

func TestGetUnusedPIDDoesNotFail(t *testing.T) {
	// An unused PID must degrade to an empty name: PID attribution is best
	// effort and a socket can outlive the process that owned it.
	proc, err := Get(0xFFFFFFFE)
	if err != nil {
		t.Fatalf("Get(0xFFFFFFFE) returned error: %v", err)
	}
	if proc.PID != 0xFFFFFFFE {
		t.Errorf("PID = %d, want 0xFFFFFFFE", proc.PID)
	}
}

func TestGetIdleProcessIsNotAnError(t *testing.T) {
	// PID 0 cannot be opened on Windows; it must degrade, not fail.
	proc, err := Get(0)
	if err != nil {
		t.Fatalf("Get(0) returned error: %v", err)
	}
	if proc.PID != 0 {
		t.Errorf("PID = %d, want 0", proc.PID)
	}
	if proc.Name != "" {
		t.Errorf("Name = %q, want empty for the idle process", proc.Name)
	}
}

func TestCacheMemoisesLookups(t *testing.T) {
	cache := NewCache()
	self := currentPID()

	first := cache.Lookup(self)
	second := cache.Lookup(self)

	if first != second {
		t.Errorf("Lookup returned different values for the same PID: %+v vs %+v", first, second)
	}
	if len(cache.entries) != 1 {
		t.Errorf("cache size = %d, want 1 (the second lookup must be a hit)", len(cache.entries))
	}
}

func TestCacheRemembersFailures(t *testing.T) {
	cache := NewCache()

	// A PID that cannot be opened must still be cached, so that the failure is
	// not retried once per socket.
	if got := cache.Lookup(0); got.PID != 0 {
		t.Errorf("PID = %d, want 0", got.PID)
	}
	if len(cache.entries) != 1 {
		t.Errorf("cache size = %d, want 1", len(cache.entries))
	}
}

func TestCacheInstancesAreIndependent(t *testing.T) {
	if got := NewCache().Lookup(0); got.PID != 0 {
		t.Errorf("PID = %d, want 0", got.PID)
	}
	if other := NewCache(); other.Len() != 0 {
		t.Errorf("a fresh cache is not empty: %d entries", other.Len())
	}
}
