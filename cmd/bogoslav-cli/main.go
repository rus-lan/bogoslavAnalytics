// Command bogoslav-cli is the deterministic command-line surface over
// internal/app (TZ.md section 7.3): every command mirrors one
// bogoslav-mcp tool and calls exactly one app function, so the CLI and
// the MCP server can never drift into two implementations of the same
// use case.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rus-lan/bogoslavAnalytics/internal/clitree"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := clitree.NewRootCmd().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
