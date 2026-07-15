package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/cache"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// GetCommentsRequest is the input to GetComments, mirroring the
// get_comments tool / comments command surface (TZ.md section 7.2).
// Exactly one of FromArtifact and MRs must be set.
//
// UserID is already resolved: GetComments never calls
// gitlab.ResolveUserID itself. Username resolution (TZ.md section 5.0)
// is FindMRs's job; a caller that only has a raw --user string calls
// ResolveUser once before building this request, so one pipeline run
// never resolves the same username twice.
//
// GetComments is cached exactly like FindMRs (TZ.md section 4: "every
// artifact IS a cache"): the query -- user, dates, and the resolved
// merge request set -- is hashed, and a fresh existing artifact-2 is
// read back instead of re-fetching. This matters most here, since
// fetching is one /discussions call per merge request -- the single
// most call-expensive step in the whole pipeline.
type GetCommentsRequest struct {
	GitlabURL string
	UserID    int64
	From      domain.Date
	To        domain.Date

	// FromArtifact is the path to an mr_list artifact whose Items become
	// the merge request set; mutually exclusive with MRs.
	FromArtifact string
	// MRs is an explicit merge request list, for a point project/MR
	// fetch that never ran find_mrs (TZ.md section 1.2); mutually
	// exclusive with FromArtifact.
	MRs []artifact.MRRef

	Dir    string
	Format artifact.Format
	Cache  CacheOptions
	// Now overrides the wall clock used to stamp source.fetched_at and to
	// judge cache freshness. Nil means time.Now.
	Now func() time.Time
}

// GetCommentsResult is the output of GetComments: the written (or, on a
// cache hit, the already-existing) artifact-2, its path, and whether the
// request was served from cache without calling GitLab.
type GetCommentsResult struct {
	Doc      artifact.CommentList
	Path     string
	CacheHit bool
}

// GetComments is the shared implementation behind the get_comments MCP
// tool and the comments CLI command (TZ.md section 7.2): resolve the
// merge request set (from an mr_list artifact or an explicit list),
// serve a fresh cached artifact-2 if one already exists, or fetch every
// discussion via gitlab.Discussions, keep only req.UserID's own
// non-system notes inside [From, To] (the same exact-match rule TZ.md
// section 5.4 pins for comment counting), and write a new artifact-2.
//
// Resolving the merge request set (reading FromArtifact, if set) is a
// local file read, not a GitLab call, so it always happens before the
// cache check: the resolved set is part of what gets hashed.
func GetComments(ctx context.Context, client DiscussionsClient, req GetCommentsRequest) (GetCommentsResult, error) {
	refs, err := resolveMRRefs(req.FromArtifact, req.MRs)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	dateRange, err := domain.NewDateRange(req.From, req.To)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	query := artifact.CommentQuery{
		UserID:       req.UserID,
		From:         req.From,
		To:           req.To,
		MRs:          refs,
		FromArtifact: req.FromArtifact,
	}

	hash, err := cache.Hash(query)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	dir := outDir(req.Dir)
	format := outFormat(req.Format)
	now := clockOrDefault(req.Now)

	path, hit, err := cache.Lookup(
		string(artifact.KindCommentList), hash,
		cache.Options{Dir: dir, TTL: req.Cache.ttl(), Refresh: req.Cache.Refresh},
		&artifact.HeaderStore{}, now(),
	)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}
	if hit {
		doc, err := artifact.ReadCommentList(path)
		if err != nil {
			return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
		}
		return GetCommentsResult{Doc: doc, Path: path, CacheHit: true}, nil
	}

	var items []artifact.CommentItem
	for _, ref := range refs {
		discussions, err := client.Discussions(ctx, gitlab.NumericID(ref.ProjectID), ref.MRIID)
		if err != nil {
			return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
		}
		for _, note := range extractUserNotes(discussions, req.UserID, dateRange) {
			items = append(items, artifact.CommentItem{MRIID: ref.MRIID, Note: note})
		}
	}

	writePath, err := artifactPath(dir, artifact.KindCommentList, hash, format)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	doc := artifact.CommentList{
		Header: artifact.Header{
			SchemaVersion: artifact.CurrentSchemaVersion,
			Kind:          artifact.KindCommentList,
			Source:        artifact.Source{GitlabURL: req.GitlabURL, FetchedAt: now()},
		},
		Query: query,
		Items: items,
	}
	if err := artifact.WriteCommentList(doc, format, writePath); err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	return GetCommentsResult{Doc: doc, Path: writePath, CacheHit: false}, nil
}

// resolveMRRefs picks the merge request set GetComments fetches comments
// for: fromArtifact's own mr_list Items when set, or explicit otherwise.
// Exactly one of the two must be set.
func resolveMRRefs(fromArtifact string, explicit []artifact.MRRef) ([]artifact.MRRef, error) {
	switch {
	case fromArtifact != "" && len(explicit) > 0:
		return nil, ErrAmbiguousMergeRequests
	case fromArtifact != "":
		doc, err := artifact.ReadMRList(fromArtifact)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", fromArtifact, err)
		}
		refs := make([]artifact.MRRef, len(doc.Items))
		for i, it := range doc.Items {
			refs[i] = artifact.MRRef{ProjectID: it.ProjectID, MRIID: it.MRIID}
		}
		return refs, nil
	case len(explicit) > 0:
		return explicit, nil
	default:
		return nil, ErrNoMergeRequests
	}
}

// extractUserNotes keeps only userID's own non-system notes that fall
// inside r -- TZ.md section 5.4's exact-match rule, the same predicate
// search.CountComments applies to build a count, applied here to the
// full domain.Note values instead of just counting them.
func extractUserNotes(discussions []domain.Discussion, userID int64, r domain.DateRange) []domain.Note {
	var out []domain.Note
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.Author.ID != userID || n.System {
				continue
			}
			if !r.Contains(n.CreatedAt) {
				continue
			}
			out = append(out, n)
		}
	}
	return out
}
