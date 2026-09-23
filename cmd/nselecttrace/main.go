// Command nselecttrace is a lightweight terminal network diagnostic and inspection tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"nselecttrace/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := cli.New(os.Stdout, os.Stderr)
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		stop() // restore the default signal behaviour before reporting the failure
		fmt.Fprintf(os.Stderr, "nselecttrace: %v\n", err)
		os.Exit(1)
	}
}
