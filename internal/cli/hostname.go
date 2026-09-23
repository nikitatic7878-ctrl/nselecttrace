package cli

import "os"

// osHostname is indirection over os.Hostname so that tests can rely on a
// stable value.
var osHostname = os.Hostname
