package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/mcptool"
)

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
func (s *toolServer) getStats(_ context.Context, _ *mcp.CallToolRequest, in mcptool.GetStatsInput) (*mcp.CallToolResult, mcptool.GetStatsOutput, error) {
	req, err := newGetStatsRequest(in)
	if err != nil {
		return nil, mcptool.GetStatsOutput{}, err
	}

	result, err := app.GetStats(req)
	if err != nil {
		return nil, mcptool.GetStatsOutput{}, fmt.Errorf("get_stats: %w", err)
	}

	return nil, mcptool.GetStatsOutput{Stats: result.Stats, Path: result.Path}, nil
}
