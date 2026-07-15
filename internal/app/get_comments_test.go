package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

func TestGetComments_fromArtifactFiltersToUserAndRangeAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	mrList := artifact.MRList{
		Header: artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: at}},
		Query: domain.Query{
			GitlabURL: "https://gitlab.example.com",
			UserID:    42,
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
		},
		Items: []artifact.MRItem{{ProjectID: 1, MRIID: 7, CommentCount: 2}},
	}
	mrListPath := filepath.Join(dir, "mr_list_test.yaml")
	if err := artifact.WriteMRList(mrList, artifact.FormatYAML, mrListPath); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	discussions := []domain.Discussion{
		discussion("d1",
			note(1, 42, false, at),                  // kept
			note(2, 99, false, at),                  // other author, dropped
			note(3, 42, true, at),                   // system note, dropped
			note(4, 42, false, at.AddDate(1, 0, 0)), // out of range, dropped
		),
	}
	client := &fakeDiscussionsClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			if project.String() != "1" || mrIID != 7 {
				t.Fatalf("Discussions(%s, %d), want (1, 7)", project.String(), mrIID)
			}
			return discussions, nil
		},
	}

	req := GetCommentsRequest{
		GitlabURL:    "https://gitlab.example.com",
		User:         "42",
		From:         domain.NewDate(2026, time.January, 1),
		To:           domain.NewDate(2026, time.June, 30),
		FromArtifact: mrListPath,
		Dir:          dir,
		Format:       artifact.FormatJSON,
	}

	result, err := GetComments(context.Background(), client, req)
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}
	if len(result.Doc.Items) != 1 || result.Doc.Items[0].ID != 1 {
		t.Fatalf("GetComments() items = %+v, want exactly note id 1", result.Doc.Items)
	}
	if result.Doc.Items[0].MRIID != 7 {
		t.Errorf("Items[0].MRIID = %d, want 7", result.Doc.Items[0].MRIID)
	}

	got, err := artifact.ReadCommentList(result.Path)
	if err != nil {
		t.Fatalf("ReadCommentList() error = %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != 1 {
		t.Errorf("ReadCommentList() items = %+v, want exactly note id 1", got.Items)
	}
}

func TestGetComments_explicitMRList(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	client := &fakeDiscussionsClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}
	req := GetCommentsRequest{
		User: "42",
		From: domain.NewDate(2026, time.January, 1),
		To:   domain.NewDate(2026, time.June, 30),
		MRs:  []artifact.MRRef{{ProjectID: 1, MRIID: 7}, {ProjectID: 2, MRIID: 8}},
		Dir:  dir,
	}

	result, err := GetComments(context.Background(), client, req)
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}
	if client.calls != 2 {
		t.Errorf("Discussions called %d times, want 2", client.calls)
	}
	if len(result.Doc.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Doc.Items))
	}
}

// TestGetComments_fetchedAtIsAlwaysUTC is the regression guard for TZ.md
// section 4.1's fetched_at contract (a "Z" instant, not a local offset):
// even when req.Now returns a non-UTC clock reading, the written
// artifact's Source.FetchedAt must carry UTC. This fails if the .UTC()
// call at the Source{} site is reverted.
func TestGetComments_fetchedAtIsAlwaysUTC(t *testing.T) {
	dir := t.TempDir()
	loc := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, time.July, 15, 23, 14, 35, 0, loc)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	client := &fakeDiscussionsClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}
	req := GetCommentsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	result, err := GetComments(context.Background(), client, req)
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}

	fetchedAt := result.Doc.Header.Source.FetchedAt
	if fetchedAt.Location() != time.UTC {
		t.Errorf("FetchedAt.Location() = %v, want time.UTC", fetchedAt.Location())
	}
	if !fetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want the same instant as %v", fetchedAt, now)
	}
}

func TestGetComments_noInputIsError(t *testing.T) {
	_, err := GetComments(context.Background(), &fakeDiscussionsClient{}, GetCommentsRequest{
		User: "42",
		From: domain.NewDate(2026, time.January, 1),
		To:   domain.NewDate(2026, time.June, 30),
	})
	if !errors.Is(err, ErrNoMergeRequests) {
		t.Errorf("error = %v, want ErrNoMergeRequests", err)
	}
}

func TestGetComments_ambiguousInputIsError(t *testing.T) {
	_, err := GetComments(context.Background(), &fakeDiscussionsClient{}, GetCommentsRequest{
		User:         "42",
		From:         domain.NewDate(2026, time.January, 1),
		To:           domain.NewDate(2026, time.June, 30),
		FromArtifact: "x.yaml",
		MRs:          []artifact.MRRef{{ProjectID: 1, MRIID: 1}},
	})
	if !errors.Is(err, ErrAmbiguousMergeRequests) {
		t.Errorf("error = %v, want ErrAmbiguousMergeRequests", err)
	}
}

func TestGetComments_cacheHitMakesZeroDiscussionsCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := GetCommentsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	client1 := &fakeDiscussionsClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}
	result1, err := GetComments(context.Background(), client1, req)
	if err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}
	if result1.CacheHit {
		t.Fatalf("GetComments() first call CacheHit = true, want false")
	}

	client2 := &fakeDiscussionsClient{} // Discussions unconfigured: panics if called
	result2, err := GetComments(context.Background(), client2, req)
	if err != nil {
		t.Fatalf("GetComments() second call error = %v", err)
	}
	if !result2.CacheHit {
		t.Fatalf("GetComments() second call CacheHit = false, want true")
	}
	if client2.calls != 0 {
		t.Errorf("Discussions called %d times on cache hit, want 0", client2.calls)
	}
	if result2.Path != result1.Path {
		t.Errorf("path = %q, want %q", result2.Path, result1.Path)
	}
}

func TestGetComments_refreshForcesMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	baseReq := GetCommentsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
		Dir:       dir,
		Now:       func() time.Time { return now },
	}
	makeClient := func() *fakeDiscussionsClient {
		return &fakeDiscussionsClient{
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
			},
		}
	}

	if _, err := GetComments(context.Background(), makeClient(), baseReq); err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}

	refreshReq := baseReq
	refreshReq.Cache = CacheOptions{Refresh: true}
	client2 := makeClient()
	result, err := GetComments(context.Background(), client2, refreshReq)
	if err != nil {
		t.Fatalf("GetComments() refresh call error = %v", err)
	}
	if result.CacheHit {
		t.Errorf("GetComments() with Refresh=true CacheHit = true, want false")
	}
	if client2.calls != 1 {
		t.Errorf("Discussions called %d times on refresh, want 1", client2.calls)
	}
}

func TestGetComments_expiredEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	baseReq := GetCommentsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
		Dir:       dir,
		Cache:     CacheOptions{TTL: time.Hour},
	}
	makeClient := func() *fakeDiscussionsClient {
		return &fakeDiscussionsClient{
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
			},
		}
	}

	req1 := baseReq
	req1.Now = func() time.Time { return t0 }
	if _, err := GetComments(context.Background(), makeClient(), req1); err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}

	req2 := baseReq
	req2.Now = func() time.Time { return t0.Add(2 * time.Hour) }
	client2 := makeClient()
	result, err := GetComments(context.Background(), client2, req2)
	if err != nil {
		t.Fatalf("GetComments() expired call error = %v", err)
	}
	if result.CacheHit {
		t.Errorf("GetComments() after TTL expiry CacheHit = true, want false")
	}
}

// TestGetComments_artifactCacheHitWithUsernameMakesZeroGitLabCalls is the
// regression this wiring closes (TZ.md sections 5.0, 14 item 15): before
// ResolveUserCached was wired into GetComments, req.User being a username
// meant every artifact-2 cache hit still paid for one
// GET /users?username= call, because app.ResolveUser ran in the caller
// before GetComments (and its cache.Lookup) ever ran. With the
// resolved-user cache in place, a second call with the same
// username-bearing request makes zero GitLab calls of any kind.
func TestGetComments_artifactCacheHitWithUsernameMakesZeroGitLabCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := GetCommentsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "alice",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	client1 := &fakeDiscussionsClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			return 42, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}
	result1, err := GetComments(context.Background(), client1, req)
	if err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}
	if result1.CacheHit {
		t.Fatalf("GetComments() first call CacheHit = true, want false")
	}
	if client1.resolveUserIDCalls != 1 {
		t.Fatalf("GetComments() first call made %d ResolveUserID calls, want 1", client1.resolveUserIDCalls)
	}

	// Every method left nil: fakeDiscussionsClient panics on any call, so a
	// second GetComments run with the same username-bearing request proves
	// this is a true zero-GitLab-call cache hit -- both the resolved-user
	// cache and the artifact-2 cache have to hit for this call to return at
	// all.
	client2 := &fakeDiscussionsClient{}
	result2, err := GetComments(context.Background(), client2, req)
	if err != nil {
		t.Fatalf("GetComments() second call error = %v", err)
	}
	if !result2.CacheHit {
		t.Fatalf("GetComments() second call CacheHit = false, want true")
	}
	if client2.resolveUserIDCalls != 0 {
		t.Errorf("GetComments() second call (artifact + resolved-user cache hit) made %d ResolveUserID calls, want 0", client2.resolveUserIDCalls)
	}
	if client2.calls != 0 {
		t.Errorf("GetComments() second call made %d Discussions calls, want 0", client2.calls)
	}
}

// TestGetComments_resolvedUserCacheExpiredEntryMakesOneMoreResolveCall
// uses a different MR iid across the two calls, on purpose: that makes
// the second call miss the artifact-2 cache regardless, so the extra
// ResolveUserID call it asserts can only come from the resolved-user
// cache's own TTL expiry, not from the artifact-2 cache never having been
// consulted in the first place.
func TestGetComments_resolvedUserCacheExpiredEntryMakesOneMoreResolveCall(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	makeReq := func(mrIID int64, now time.Time) GetCommentsRequest {
		return GetCommentsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "alice",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: mrIID}},
			Dir:       dir,
			Cache:     CacheOptions{TTL: time.Hour},
			Now:       func() time.Time { return now },
		}
	}
	client := &fakeDiscussionsClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			return 42, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}

	if _, err := GetComments(context.Background(), client, makeReq(7, t0)); err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}
	if client.resolveUserIDCalls != 1 {
		t.Fatalf("GetComments() first call made %d ResolveUserID calls, want 1", client.resolveUserIDCalls)
	}

	if _, err := GetComments(context.Background(), client, makeReq(8, t0.Add(2*time.Hour))); err != nil {
		t.Fatalf("GetComments() expired call error = %v", err)
	}
	if client.resolveUserIDCalls != 2 {
		t.Errorf("GetComments() after resolved-user cache TTL expiry made %d total ResolveUserID calls, want 2", client.resolveUserIDCalls)
	}
}

// TestGetComments_resolvedUserCacheRefreshMakesOneMoreResolveCall mirrors
// TestGetComments_resolvedUserCacheExpiredEntryMakesOneMoreResolveCall,
// forcing the extra ResolveUserID call with --refresh instead of TTL
// expiry.
func TestGetComments_resolvedUserCacheRefreshMakesOneMoreResolveCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	makeReq := func(mrIID int64, refresh bool) GetCommentsRequest {
		return GetCommentsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "alice",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: mrIID}},
			Dir:       dir,
			Cache:     CacheOptions{Refresh: refresh},
			Now:       func() time.Time { return now },
		}
	}
	client := &fakeDiscussionsClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			return 42, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}

	if _, err := GetComments(context.Background(), client, makeReq(7, false)); err != nil {
		t.Fatalf("GetComments() first call error = %v", err)
	}
	if _, err := GetComments(context.Background(), client, makeReq(8, true)); err != nil {
		t.Fatalf("GetComments() refresh call error = %v", err)
	}
	if client.resolveUserIDCalls != 2 {
		t.Errorf("GetComments() with Refresh=true made %d total ResolveUserID calls, want 2", client.resolveUserIDCalls)
	}
}

// TestGetComments_numericUserMakesNoResolveCalls proves a numeric --user
// never touches the resolved-user cache, or ResolveUserID, regardless of
// how many times GetComments runs.
func TestGetComments_numericUserMakesNoResolveCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	client := &fakeDiscussionsClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
		},
	}
	for i, mrIID := range []int64{7, 8, 9} {
		req := GetCommentsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: mrIID}},
			Dir:       dir,
			Now:       func() time.Time { return now },
		}
		if _, err := GetComments(context.Background(), client, req); err != nil {
			t.Fatalf("GetComments() call %d error = %v", i, err)
		}
	}
	if client.resolveUserIDCalls != 0 {
		t.Errorf("GetComments() with numeric user made %d ResolveUserID calls, want 0", client.resolveUserIDCalls)
	}
}

// TestGetComments_resolvedUserCacheKeyIncludesGitlabURL is the hazard
// ResolveUserCached's doc comment names first (user.go), exercised
// through GetComments's own wiring: the same username on two different
// GitLab instances is two different people, so a resolution on one
// instance must never answer for the other.
func TestGetComments_resolvedUserCacheKeyIncludesGitlabURL(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	makeReq := func(gitlabURL string) GetCommentsRequest {
		return GetCommentsRequest{
			GitlabURL: gitlabURL,
			User:      "alice",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
			Dir:       dir,
			Now:       func() time.Time { return now },
		}
	}
	client := &fakeDiscussionsClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			return 99, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return []domain.Discussion{discussion("d", note(1, 99, false, at))}, nil
		},
	}

	if _, err := GetComments(context.Background(), client, makeReq("https://gitlab-a.example.com")); err != nil {
		t.Fatalf("GetComments() first instance error = %v", err)
	}
	if client.resolveUserIDCalls != 1 {
		t.Fatalf("GetComments() first instance made %d ResolveUserID calls, want 1", client.resolveUserIDCalls)
	}

	if _, err := GetComments(context.Background(), client, makeReq("https://gitlab-b.example.com")); err != nil {
		t.Fatalf("GetComments() second instance error = %v", err)
	}
	if client.resolveUserIDCalls != 2 {
		t.Errorf("GetComments() same username on a different gitlab_url made %d total ResolveUserID calls, want 2 (independent resolution)", client.resolveUserIDCalls)
	}
}
