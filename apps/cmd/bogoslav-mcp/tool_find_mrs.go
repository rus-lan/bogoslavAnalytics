package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/mcptool"
)

// newFindMRsRequest converts in into an app.FindMRsRequest. It makes no
// GitLab call and does not resolve user: app.FindMRs does both itself
// (TZ.md section 5.0), so this mapping stays pure and testable on its
// own, mirroring bogoslav-cli's newFindMRsRequest.
func newFindMRsRequest(in mcptool.FindMRsInput, gitlabURL string) (app.FindMRsRequest, error) {
	from, err := domain.ParseDate(in.From)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("from: %w", err)
	}
	to, err := domain.ParseDate(in.To)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("to: %w", err)
	}
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.FindMRsRequest{}, err
	}

	req := app.FindMRsRequest{
		GitlabURL: gitlabURL,
		User:      in.User,
		From:      from,
		To:        to,
		MoreThan:  in.MoreThan,
		Group:     in.Group,
		Project:   in.Project,
		Strict:    in.Strict,
		Dir:       in.ArtifactsDir,
		Format:    format,
		Cache:     cacheOptions(in.Refresh, in.CacheTTLSeconds),
	}
	if in.MR != 0 {
		mr := in.MR
		req.MR = &mr
	}
	return req, nil
}

// findMRs is the find_mrs tool handler: build the request and call
// app.FindMRs (TZ.md section 7.3: one function of apps/internal/app per
// tool). Point mode's "mr requires project" rule (TZ.md sections 1.2,
// 7.2) is enforced by app.FindMRs itself, not here.
func (s *toolServer) findMRs(ctx context.Context, _ *mcp.CallToolRequest, in mcptool.FindMRsInput) (*mcp.CallToolResult, mcptool.FindMRsOutput, error) {
	req, err := newFindMRsRequest(in, s.gitlabURL)
	if err != nil {
		return nil, mcptool.FindMRsOutput{}, err
	}

	result, err := app.FindMRs(ctx, s.client, req)
	if err != nil {
		return nil, mcptool.FindMRsOutput{}, fmt.Errorf("find_mrs: %w", err)
	}

	return nil, mcptool.FindMRsOutput{
		Path:     result.Path,
		CacheHit: result.CacheHit,
		Count:    len(result.Doc.Items),
		Strategy: string(result.Doc.Query.Strategy),
		Smoke:    string(result.Doc.Query.Smoke),
	}, nil
}
