package cli

import (
	"context"
	"fmt"

	"nselecttrace/internal/buildinfo"
)

// init registers the commands that exist today. Commands for tcp and trace are
// appended here as they land, each in its own file, without touching main.go or
// the App itself.
func (a *App) init() {
	a.Add(
		a.aboutCommand(),
		a.versionCommand(),
		a.interfacesCommand(),
		a.portsCommand(),
		a.connectionsCommand(),
		a.dnsCommand(),
		a.pingCommand(),
	)
}

// aboutCommand prints a short banner. It is also the action taken when nselecttrace
// is started without arguments, so the bare invocation is useful.
func (a *App) aboutCommand() Command {
	return Command{
		Name:    "about",
		Summary: "show a short description of nselecttrace",
		Usage:   "about",
		Run: func(_ context.Context, _ []string) error {
			fmt.Fprintf(a.stdout, "%s\n", programName)
			fmt.Fprintf(a.stdout, "Network diagnostic utility\n\n")
			fmt.Fprintf(a.stdout, "host:    %s\n", hostName())
			fmt.Fprintf(a.stdout, "runtime: %s\n", runtimeInfo())
			fmt.Fprintf(a.stdout, "version: %s\n", buildinfo.Version)
			fmt.Fprintf(a.stdout, "\nRun '%s --help' to list the available commands.\n", programName)
			return nil
		},
	}
}

// versionCommand reports the build metadata of the running binary.
func (a *App) versionCommand() Command {
	return Command{
		Name:    "version",
		Summary: "show version and build information",
		Usage:   "version",
		Run: func(_ context.Context, _ []string) error {
			fmt.Fprintf(a.stdout, "%s %s (%s, built %s)\n",
				programName, buildinfo.Version, buildinfo.Commit, buildinfo.Date)
			return nil
		},
	}
}
