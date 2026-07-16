package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// GetCommentsRequest is the input to GetComments, mirroring the
// get_comments tool / comments command surface (TZ.md section 7.2).
// Exactly one of FromArtifact and MRs must be set.
//
// User is a numeric id or a username (TZ.md section 5.0); GetComments
// resolves it exactly once via ResolveUserCached -- the same
// {gitlab_url, user} on-disk cache FindMRs uses (user.go), so a username
// costs one GET /users?username= call and then comes from that cache
// until the TTL expires or --refresh forces a miss, and a numeric User
// costs zero calls, always. See ResolveUserCached's doc comment for the
// rename hazard this cache carries: it now applies here too, not only to
// find_mrs.
//
// GetComments is cached exactly like FindMRs (TZ.md section 4: "every
// artifact IS a cache"): the query -- user, dates, and the resolved
// merge request set -- is hashed together with ToolVersion via
// cache.HashWithToolVersion, and a fresh existing artifact-2 is read
// back instead of re-fetching, unless it was written under a different
// tool version (TZ.md section 4.6: the same real incident FindMRs's
// cache.QueryHash guards against applies here too -- a comment_list
// artifact an old, buggy binary already wrote must not go on answering
// an identical query from a fixed binary for the rest of its TTL).
// This matters most here, since fetching is one /discussions call per
// merge request -- the single most call-expensive step in the whole
// pipeline.
type GetCommentsRequest struct {
	GitlabURL string
	// User is a numeric id or a username (TZ.md section 5.0); GetComments
	// resolves it exactly once via ResolveUserCached.
	User string
	From domain.Date
	To   domain.Date

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
// tool and the comments CLI command (TZ.md section 7.2): resolve --user
// (cached, see ResolveUserCached) and the merge request set (from an
// mr_list artifact or an explicit list), serve a fresh cached artifact-2
// if one already exists, or fetch every discussion via
// gitlab.Discussions, keep only the resolved user's own non-system notes
// inside [From, To] (the same exact-match rule TZ.md section 5.4 pins
// for comment counting), and write a new artifact-2.
//
// Resolving the merge request set (reading FromArtifact, if set) is a
// local file read, not a GitLab call, so it always happens first, before
// --user is resolved or any cache is consulted.
func GetComments(ctx context.Context, client GetCommentsClient, req GetCommentsRequest) (GetCommentsResult, error) {
	refs, err := resolveMRRefs(req.FromArtifact, req.MRs)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	dir := outDir(req.Dir)
	format := outFormat(req.Format)
	now := clockOrDefault(req.Now)
	cacheOpts := cache.Options{Dir: dir, TTL: req.Cache.ttl(), Refresh: req.Cache.Refresh}

	// gitlabURL is the credential-free copy of req.GitlabURL used for
	// every provenance/cache-key purpose below (see sanitizeGitlabURL's
	// doc comment, gitlaburl.go). The real GitLab request never goes
	// through this value: client was already built from the raw,
	// credentialed GITLAB_URL by the caller.
	gitlabURL := sanitizeGitlabURL(req.GitlabURL)

	userID, err := ResolveUserCached(ctx, client, gitlabURL, req.User, cacheOpts, now())
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	dateRange, err := domain.NewDateRange(req.From, req.To)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	query := artifact.CommentQuery{
		UserID:       userID,
		From:         req.From,
		To:           req.To,
		MRs:          refs,
		FromArtifact: req.FromArtifact,
	}

	hash, err := cache.HashWithToolVersion(query, ToolVersion)
	if err != nil {
		return GetCommentsResult{}, fmt.Errorf("get comments: %w", err)
	}

	path, hit, err := cache.Lookup(
		string(artifact.KindCommentList), hash,
		cacheOpts,
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
		for _, note := range extractUserNotes(discussions, userID, dateRange) {
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
			Source:        artifact.Source{GitlabURL: gitlabURL, FetchedAt: now().UTC()},
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
