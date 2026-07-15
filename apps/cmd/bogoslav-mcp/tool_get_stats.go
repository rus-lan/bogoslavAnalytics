package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/stats"
)

// GetStatsInput is the get_stats tool's input: the MCP mirror of
// bogoslav-cli's get-stats command and app.GetStatsRequest (TZ.md
// sections 7.2, 7.2.1, 7.3). get_stats never calls GitLab: it only
// aggregates an already-written artifact's items.
type GetStatsInput struct {
	ArtifactPath string `json:"artifact_path" jsonschema:"path to any one of the four artifact kinds (mr_list, comment_list, labeled_comments, filtered_comments) to aggregate"`
	ArtifactsDir string `json:"artifacts_dir,omitempty" jsonschema:"when set, also writes the aggregate as a stats_<name>.<ext> file (json or yaml only) under this directory; omit to only return the aggregate without writing a file"`
	Format       string `json:"format,omitempty" jsonschema:"output format for the written stats file when artifacts_dir is set: json or yaml only (default yaml); stats is not one of the four artifact kinds and has no text or html rendering of its own"`
}

// GetStatsOutput is the get_stats tool's output.
type GetStatsOutput struct {
	Stats stats.Stats `json:"stats" jsonschema:"the aggregate: total item count and, depending on the input artifact's kind, breakdowns by merge request, by label, and by day"`
	Path  string      `json:"path,omitempty" jsonschema:"path the aggregate was written to; set only when artifacts_dir was given"`
}

// newGetStatsRequest converts in into an app.GetStatsRequest.
func newGetStatsRequest(in GetStatsInput) (app.GetStatsRequest, error) {
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.GetStatsRequest{}, err
	}
	return app.GetStatsRequest{
		ArtifactPath: in.ArtifactPath,
		Dir:          in.ArtifactsDir,
		Format:       format,
	}, nil
}

// getStats is the get_stats tool handler: build the request and call
// app.GetStats (TZ.md section 7.3: one function of apps/internal/app
// per tool).
func (s *toolServer) getStats(_ context.Context, _ *mcp.CallToolRequest, in GetStatsInput) (*mcp.CallToolResult, GetStatsOutput, error) {
	req, err := newGetStatsRequest(in)
	if err != nil {
		return nil, GetStatsOutput{}, err
	}

	result, err := app.GetStats(req)
	if err != nil {
		return nil, GetStatsOutput{}, fmt.Errorf("get_stats: %w", err)
	}

	return nil, GetStatsOutput{Stats: result.Stats, Path: result.Path}, nil
}
