package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/mcptool"
)

// newGetCommentsRequest converts in into an app.GetCommentsRequest. It
// makes no GitLab call and does not resolve user: app.GetComments does
// both itself (TZ.md section 5.0), so this mapping stays pure and
// testable on its own, mirroring bogoslav-cli's newGetCommentsRequest.
func newGetCommentsRequest(in mcptool.GetCommentsInput, gitlabURL string) (app.GetCommentsRequest, error) {
	from, err := domain.ParseDate(in.From)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("from: %w", err)
	}
	to, err := domain.ParseDate(in.To)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("to: %w", err)
	}
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.GetCommentsRequest{}, err
	}

	return app.GetCommentsRequest{
		GitlabURL:    gitlabURL,
		User:         in.User,
		From:         from,
		To:           to,
		FromArtifact: in.FromArtifact,
		MRs:          in.MRs,
		Dir:          in.ArtifactsDir,
		Format:       format,
		Cache:        cacheOptions(in.Refresh, in.CacheTTLSeconds),
	}, nil
}

// getComments is the get_comments tool handler: build the request and
// call app.GetComments (TZ.md section 7.3: one function of
// apps/internal/app per tool). User resolution (TZ.md section 5.0) is
// app.GetComments's own job, not this handler's.
func (s *toolServer) getComments(ctx context.Context, _ *mcp.CallToolRequest, in mcptool.GetCommentsInput) (*mcp.CallToolResult, mcptool.GetCommentsOutput, error) {
	req, err := newGetCommentsRequest(in, s.gitlabURL)
	if err != nil {
		return nil, mcptool.GetCommentsOutput{}, err
	}

	result, err := app.GetComments(ctx, s.client, req)
	if err != nil {
		return nil, mcptool.GetCommentsOutput{}, fmt.Errorf("get_comments: %w", err)
	}

	return nil, mcptool.GetCommentsOutput{
		Path:     result.Path,
		CacheHit: result.CacheHit,
		Count:    len(result.Doc.Items),
	}, nil
}
