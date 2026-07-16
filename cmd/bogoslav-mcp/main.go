package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/clitree"
)

// main runs bogoslav-mcp on stdio (TZ.md section 7.1). Diagnostics go to
// stderr only via the slog.Logger built here: stdout carries the MCP
// protocol stream exclusively, and nothing in this package ever writes
// to it directly -- including `version` (TZ.md section 7.4), which
// writes through cmd.OutOrStdout() (internal/clitree.NewVersionCmd()),
// never os.Stdout or fmt.Print* by name, so it does not trip
// TestPackage_neverWritesToStdoutDirectly (stdout_test.go) either.
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd(logger).ExecuteContext(ctx); err != nil {
		logger.Error("bogoslav-mcp exited with an error", "error", err)
		os.Exit(1)
	}
}

// newRootCmd builds bogoslav-mcp's own command surface: invoked with no
// arguments -- the only way any MCP client ever spawns this binary --
// its own RunE starts the stdio server exactly as main did before this
// command tree existed. `version` (or --version), attached via
// clitree.AddVersionSupport, prints this build's version and exits
// without ever starting the server or touching stdout except through
// cmd.OutOrStdout(). SilenceErrors/SilenceUsage keep a server-start
// failure's reporting exactly as it was: main's own logger.Error call is
// the only diagnostic written, not a second "Error: ..." line from
// cobra's own default error handling.
func newRootCmd(logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:           "bogoslav-mcp",
		Short:         "GitLab review-activity analytics MCP server, over stdio",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), logger)
		},
	}
	clitree.AddVersionSupport(root)
	return root
}

// run wires GITLAB_URL/GITLAB_TOKEN (TZ.md section 2.5) into a GitLab
// client, builds the MCP server (TZ.md section 7.3: six tools, one
// internal/app use case each), and runs it on stdio (TZ.md section
// 7.1). A missing GITLAB_TOKEN surfaces here as a returned error, not a
// panic: main logs it to stderr and exits 1.
func run(ctx context.Context, logger *slog.Logger) error {
	client, err := newGitlabClientFromEnv()
	if err != nil {
		return fmt.Errorf("bogoslav-mcp: %w", err)
	}

	server := newServer(client, resolvedGitlabURL(), logger)
	return server.Run(ctx, &mcp.StdioTransport{})
}
