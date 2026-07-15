package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// main runs bogoslav-mcp on stdio (TZ.md section 7.1). Diagnostics go to
// stderr only via the slog.Logger built here: stdout carries the MCP
// protocol stream exclusively, and nothing in this package ever writes
// to it directly.
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("bogoslav-mcp exited with an error", "error", err)
		os.Exit(1)
	}
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
