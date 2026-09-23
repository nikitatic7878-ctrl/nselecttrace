package processes

import "os"

// currentPID is the PID of the running test binary. Tests use it instead of a
// well-known system PID so they do not depend on what else runs on the machine.
func currentPID() uint32 { return uint32(os.Getpid()) }
