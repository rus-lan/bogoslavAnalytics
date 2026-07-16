// Command bogoslav-cli is the deterministic command-line surface over
// internal/app (TZ.md section 7.3): every one of clitree.NewRootCmd's
// six pipeline commands mirrors one bogoslav-mcp tool and calls exactly
// one app function, so the CLI and the MCP server can never drift into
// two implementations of the same use case. `version` (TZ.md section
// 7.4), attached here rather than inside NewRootCmd itself, is the one
// deliberate exception: it mirrors no app function and no MCP tool.
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

	root := clitree.NewRootCmd()
	clitree.AddVersionSupport(root)

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
