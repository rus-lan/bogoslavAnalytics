package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/mcptool"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/stats"
)

// GetStatsOutput is the get_stats tool's output.
type GetStatsOutput struct {
	Stats stats.Stats `json:"stats" jsonschema:"the aggregate: total item count and, depending on the input artifact's kind, breakdowns by merge request, by label, and by day"`
	Path  string      `json:"path,omitempty" jsonschema:"path the aggregate was written to; set only when artifacts_dir was given"`
}

// newGetStatsRequest converts in into an app.GetStatsRequest.
func newGetStatsRequest(in mcptool.GetStatsInput) (app.GetStatsRequest, error) {
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
func (s *toolServer) getStats(_ context.Context, _ *mcp.CallToolRequest, in mcptool.GetStatsInput) (*mcp.CallToolResult, GetStatsOutput, error) {
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
