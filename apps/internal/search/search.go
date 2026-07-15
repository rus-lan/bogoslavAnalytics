package search

import (
	"context"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// Find runs the full merge request search pipeline of TZ.md section 5:
// pick a strategy (SelectStrategy), collect that strategy's candidate
// merge requests, compute each candidate's exact comment count
// (CountComments), and keep only the candidates whose exact count is
// strictly greater than p.MoreThan -- the pinned boundary of TZ.md section
// 4.1. Candidate pre-filters (the events preliminary count and the
// bruteforce user_notes_count check) use ">=" as a deliberate superset;
// this final filter is the only place ">" is enforced.
//
// Find does not resolve --user or a --group/--project path to a numeric
// id, and it does not implement the point mode of TZ.md section 7.2
// (project+mr with no candidate search): both are the caller's job, ahead
// of building Params.
func Find(ctx context.Context, client Client, p Params, opts Options) (Result, error) {
	strategy, smoke, err := SelectStrategy(ctx, client, p, opts)
	if err != nil {
		return Result{}, fmt.Errorf("search: find: %w", err)
	}

	var candidates []domain.MergeRequest
	switch strategy {
	case domain.StrategyEvents:
		candidates, err = eventsCandidates(ctx, client, p)
	case domain.StrategyBruteforce:
		candidates, err = bruteforceCandidates(ctx, client, p)
	default:
		return Result{}, fmt.Errorf("search: find: strategy %q: %w", strategy, ErrUnknownStrategy)
	}
	if err != nil {
		return Result{}, fmt.Errorf("search: find: %w", err)
	}

	items, err := resolveCandidates(ctx, client, candidates, p)
	if err != nil {
		return Result{}, fmt.Errorf("search: find: %w", err)
	}

	return Result{Strategy: strategy, Smoke: smoke, Items: items}, nil
}

// resolveCandidates computes the exact comment count for every candidate
// and keeps only the ones whose exact count is strictly greater than
// p.MoreThan (TZ.md section 4.1).
func resolveCandidates(ctx context.Context, client Client, candidates []domain.MergeRequest, p Params) ([]domain.MergeRequest, error) {
	var out []domain.MergeRequest
	for _, cand := range candidates {
		count, err := CountComments(ctx, client, cand.ProjectID, cand.IID, p.UserID, p.Range)
		if err != nil {
			return nil, err
		}
		if count <= p.MoreThan {
			continue
		}
		cand.CommentCount = count
		out = append(out, cand)
	}
	return out, nil
}
