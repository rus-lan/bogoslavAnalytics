package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/filter"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/search"
)

// FindMRsRequest is the input to FindMRs, mirroring the find_mrs tool /
// find command surface (TZ.md sections 7.2, 7.3).
//
// Cache key hazard (TZ.md section 4.6, accepted, not fixed here): the
// query hash cache.QueryHash builds includes Group/Project as PATH
// strings, not resolved numeric ids. If a group or project is renamed
// and a new one takes over the old path, the same cache key answers for
// the new object's data until the cached entry ages out -- bounded by
// Cache.TTL (default cache.DefaultTTL, 24h). This is an accepted risk,
// not something FindMRs works around.
//
// A second, sharper cache hazard applies when User is a username rather
// than a numeric id: see ResolveUserCached's doc comment (user.go) for
// the full explanation -- a stale entry surviving a GitLab username
// rename answers with the wrong *person*, not merely the wrong
// *selection* the hazard above describes.
type FindMRsRequest struct {
	GitlabURL string
	// User is a numeric id or a username (TZ.md section 5.0); FindMRs
	// resolves it exactly once via ResolveUserCached.
	User     string
	From     domain.Date
	To       domain.Date
	MoreThan int
	Group    string
	Project  string
	// MR, set together with Project, requests point mode (TZ.md sections
	// 1.2, 7.2, 12 criterion 26): the single named merge request, with
	// no candidate search of any kind (no events, no bruteforce, no
	// autoselector) -- just the exact /discussions count for that one
	// merge request.
	MR     *int64
	Strict bool

	Dir    string
	Format artifact.Format
	Cache  CacheOptions
	// Now overrides the wall clock used to stamp source.fetched_at, to
	// judge cache freshness, and (via search.Options.Now) to judge the
	// events-retention window. Nil means time.Now.
	Now func() time.Time
}

// FindMRsResult is the output of FindMRs: the written (or, on a cache
// hit, the already-existing) artifact-1, its path, and whether the
// request was served from cache without calling GitLab.
type FindMRsResult struct {
	Doc      artifact.MRList
	Path     string
	CacheHit bool
}

// FindMRs is the shared implementation behind the find_mrs MCP tool and
// the find CLI command (TZ.md section 7.2): resolve --user, build the
// normalized query, serve a fresh cached artifact-1 if one already
// exists, or -- in point mode (MR set) -- count that one merge request
// directly, or otherwise run search.Find, and write a new artifact-1.
//
// The comment_count > MoreThan boundary (TZ.md section 4.1) is decided
// in exactly one place regardless of path: filter.MRsByCount, applied
// once below to whatever toMRList produced. For the search.Find path
// this is a no-op (search.Find's own resolveCandidates already applied
// the identical ">" predicate before returning), but funneling both
// paths through the same call keeps there from ever being a second,
// possibly diverging, place that decides this boundary.
func FindMRs(ctx context.Context, client FindMRsClient, req FindMRsRequest) (FindMRsResult, error) {
	if req.MR != nil && req.Project == "" {
		return FindMRsResult{}, ErrPointModeRequiresProject
	}

	dir := outDir(req.Dir)
	format := outFormat(req.Format)
	now := clockOrDefault(req.Now)
	cacheOpts := cache.Options{Dir: dir, TTL: req.Cache.ttl(), Refresh: req.Cache.Refresh}

	userID, err := ResolveUserCached(ctx, client, req.GitlabURL, req.User, cacheOpts, now())
	if err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}

	dateRange, err := domain.NewDateRange(req.From, req.To)
	if err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}

	query := domain.Query{
		GitlabURL: req.GitlabURL,
		UserID:    userID,
		From:      req.From,
		To:        req.To,
		MoreThan:  req.MoreThan,
		Group:     req.Group,
		Project:   req.Project,
		MR:        req.MR,
	}

	hash, err := cache.QueryHash(query)
	if err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}

	path, hit, err := cache.Lookup(
		string(artifact.KindMRList), hash,
		cacheOpts,
		&artifact.HeaderStore{}, now(),
	)
	if err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}
	if hit {
		doc, err := artifact.ReadMRList(path)
		if err != nil {
			return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
		}
		return FindMRsResult{Doc: doc, Path: path, CacheHit: true}, nil
	}

	var result search.Result
	if req.MR != nil {
		mr, err := findPointMR(ctx, client, req.Project, *req.MR, userID, dateRange)
		if err != nil {
			return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
		}
		result = search.Result{Items: []domain.MergeRequest{mr}}
	} else {
		params := toSearchParams(query)
		// cachingSmokeClient wraps client so search.Find's one call to
		// SmokeTest (inside SelectStrategy, TZ.md section 5.3b) is served
		// from cache when a fresh entry exists, without search/ itself
		// changing at all (smoke_cache.go).
		smokeClient := &cachingSmokeClient{Client: client, gitlabURL: req.GitlabURL, opts: cacheOpts, now: now}
		result, err = search.Find(ctx, smokeClient, params, search.Options{Strict: req.Strict, Now: now})
		if err != nil {
			return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
		}
	}

	header := artifact.Header{
		SchemaVersion: artifact.CurrentSchemaVersion,
		Kind:          artifact.KindMRList,
		Source:        artifact.Source{GitlabURL: req.GitlabURL, FetchedAt: now().UTC()},
	}
	doc := toMRList(header, query, result)
	doc.Items = filter.MRsByCount(doc.Items, req.MoreThan)

	writePath, err := artifactPath(dir, artifact.KindMRList, hash, format)
	if err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}
	if err := artifact.WriteMRList(doc, format, writePath); err != nil {
		return FindMRsResult{}, fmt.Errorf("find mrs: %w", err)
	}

	return FindMRsResult{Doc: doc, Path: writePath, CacheHit: false}, nil
}

// findPointMR resolves a single (project, mr) pair to a domain.MergeRequest
// with its exact comment count, without running any candidate search
// (TZ.md sections 1.2, 7.2's point mode): it reuses
// Client.ProjectMergeRequestsByIIDs -- the same call search's events
// strategy already uses to enrich its candidates (search/enrich.go) --
// which returns the merge request's numeric ProjectID, Title, WebURL,
// CreatedAt, UpdatedAt and ProjectPath regardless of whether project was
// a path or an id, so the resulting artifact.MRItem has the exact same
// shape the events and bruteforce paths produce for the same merge
// request. The exact count itself comes from search.CountComments (TZ.md
// section 5.4), the same function every other path uses. FindMRs applies
// the comment_count > MoreThan boundary afterwards, uniformly, via
// filter.MRsByCount -- not here.
func findPointMR(ctx context.Context, client FindMRsClient, project string, mrIID, userID int64, r domain.DateRange) (domain.MergeRequest, error) {
	id := buildID(project)

	summaries, err := client.ProjectMergeRequestsByIIDs(ctx, id, []int64{mrIID})
	if err != nil {
		return domain.MergeRequest{}, fmt.Errorf("point mode: %w", err)
	}
	if len(summaries) == 0 {
		return domain.MergeRequest{}, fmt.Errorf("point mode: project %q mr %d: %w", project, mrIID, ErrMergeRequestNotFound)
	}
	mr := summaries[0].MergeRequest

	count, err := search.CountComments(ctx, client, mr.ProjectID, mrIID, userID, r)
	if err != nil {
		return domain.MergeRequest{}, fmt.Errorf("point mode: %w", err)
	}
	mr.CommentCount = count
	return mr, nil
}
