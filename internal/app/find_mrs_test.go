package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
	"github.com/rus-lan/bogoslavAnalytics/internal/search"
)

// int64Ptr returns a pointer to n, for building FindMRsRequest.MR values.
func int64Ptr(n int64) *int64 { return &n }

func TestFindMRs_cacheHitSkipsSearchAndNumericUserMakesNoResolveCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7, ProjectPath: "g/p", Title: "t", WebURL: "u", CreatedAt: at, UpdatedAt: at}, UserNotesCount: 10},
	}
	client1 := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	result1, err := FindMRs(context.Background(), client1, req)
	if err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}
	if result1.CacheHit {
		t.Fatalf("FindMRs() first call CacheHit = true, want false")
	}
	if client1.resolveUserIDCalls != 0 {
		t.Errorf("FindMRs() with numeric user called ResolveUserID %d times, want 0", client1.resolveUserIDCalls)
	}
	if len(result1.Doc.Items) != 1 || result1.Doc.Items[0].CommentCount != 4 {
		t.Fatalf("FindMRs() first call items = %+v, want one item with comment_count 4", result1.Doc.Items)
	}

	client2 := &fakeClient{} // every method nil: any call panics
	result2, err := FindMRs(context.Background(), client2, req)
	if err != nil {
		t.Fatalf("FindMRs() second call error = %v", err)
	}
	if !result2.CacheHit {
		t.Fatalf("FindMRs() second call CacheHit = false, want true")
	}
	if result2.Path != result1.Path {
		t.Errorf("FindMRs() second call path = %q, want %q", result2.Path, result1.Path)
	}
}

// TestFindMRs_fetchedAtIsAlwaysUTC is the regression guard for TZ.md
// section 4.1's fetched_at contract (a "Z" instant, not a local offset):
// even when req.Now returns a non-UTC clock reading, the written
// artifact's Source.FetchedAt must carry UTC. This fails if the .UTC()
// call at the Source{} site is reverted.
// TestFindMRs_artifactCacheHitWithUsernameMakesZeroGitLabCalls is the
// regression this wiring exists for (TZ.md sections 5.0, 14 item 15): before
// ResolveUserCached, req.User being a username meant every artifact cache
// hit still paid for one GET /users?username= call, because ResolveUser ran
// before cache.Lookup on every call. With the resolved-user cache in place,
// a second call with the same username-bearing request makes zero GitLab
// calls of any kind.
func TestFindMRs_artifactCacheHitWithUsernameMakesZeroGitLabCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "alice",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7}, UserNotesCount: 10},
	}
	client1 := &fakeClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			return 42, nil
		},
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	result1, err := FindMRs(context.Background(), client1, req)
	if err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}
	if result1.CacheHit {
		t.Fatalf("FindMRs() first call CacheHit = true, want false")
	}
	if client1.resolveUserIDCalls != 1 {
		t.Fatalf("FindMRs() first call made %d ResolveUserID calls, want 1", client1.resolveUserIDCalls)
	}

	// Every method left nil: fakeClient panics on any call, so a second
	// FindMRs run with the same username-bearing request proves this is a
	// true zero-GitLab-call cache hit -- both the resolved-user cache and
	// the artifact cache have to hit for this call to return at all.
	client2 := &fakeClient{}
	result2, err := FindMRs(context.Background(), client2, req)
	if err != nil {
		t.Fatalf("FindMRs() second call error = %v", err)
	}
	if !result2.CacheHit {
		t.Fatalf("FindMRs() second call CacheHit = false, want true")
	}
	if client2.resolveUserIDCalls != 0 {
		t.Errorf("FindMRs() second call (artifact + resolved-user cache hit) made %d ResolveUserID calls, want 0", client2.resolveUserIDCalls)
	}
}

// TestFindMRs_nonStrictSecondRunWithinTTLMakesZeroSmokeTestCalls uses a
// different MoreThan across the two calls, on purpose: that makes the
// second call miss the artifact cache and run search.Find again, so the
// zero SmokeTest calls it asserts can only come from the smoke cache, not
// from the artifact cache short-circuiting the whole request.
func TestFindMRs_nonStrictSecondRunWithinTTLMakesZeroSmokeTestCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo"}, UserNotesCount: 10},
	}
	makeReq := func(moreThan int) FindMRsRequest {
		return FindMRsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  moreThan,
			Project:   "my-group/repo",
			Dir:       dir,
			Now:       func() time.Time { return now },
		}
	}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokeFailed, nil
		},
		projectMergeRequestsFn: func(ctx context.Context, project gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	if _, err := FindMRs(context.Background(), client, makeReq(1)); err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}
	if client.smokeTestCalls != 1 {
		t.Fatalf("FindMRs() first non-strict call made %d SmokeTest calls, want 1", client.smokeTestCalls)
	}

	result, err := FindMRs(context.Background(), client, makeReq(2))
	if err != nil {
		t.Fatalf("FindMRs() second call error = %v", err)
	}
	if result.CacheHit {
		t.Fatalf("FindMRs() second call (different more_than) CacheHit = true, want false")
	}
	if client.smokeTestCalls != 1 {
		t.Errorf("FindMRs() second non-strict call within TTL made %d total SmokeTest calls, want still 1 (cached)", client.smokeTestCalls)
	}
}

func TestFindMRs_expiredSmokeCacheMakesOneMoreCall(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo"}, UserNotesCount: 10},
	}
	makeReq := func(moreThan int, now time.Time) FindMRsRequest {
		return FindMRsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  moreThan,
			Project:   "my-group/repo",
			Dir:       dir,
			Cache:     CacheOptions{TTL: time.Hour},
			Now:       func() time.Time { return now },
		}
	}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokeFailed, nil
		},
		projectMergeRequestsFn: func(ctx context.Context, project gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	if _, err := FindMRs(context.Background(), client, makeReq(1, t0)); err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}

	if _, err := FindMRs(context.Background(), client, makeReq(2, t0.Add(2*time.Hour))); err != nil {
		t.Fatalf("FindMRs() expired call error = %v", err)
	}
	if client.smokeTestCalls != 2 {
		t.Errorf("FindMRs() after smoke cache TTL expiry made %d total SmokeTest calls, want 2", client.smokeTestCalls)
	}
}

func TestFindMRs_refreshMakesOneMoreSmokeTestCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo"}, UserNotesCount: 10},
	}
	makeReq := func(moreThan int, refresh bool) FindMRsRequest {
		return FindMRsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  moreThan,
			Project:   "my-group/repo",
			Dir:       dir,
			Cache:     CacheOptions{Refresh: refresh},
			Now:       func() time.Time { return now },
		}
	}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokeFailed, nil
		},
		projectMergeRequestsFn: func(ctx context.Context, project gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	if _, err := FindMRs(context.Background(), client, makeReq(1, false)); err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}
	if _, err := FindMRs(context.Background(), client, makeReq(2, true)); err != nil {
		t.Fatalf("FindMRs() refresh call error = %v", err)
	}
	if client.smokeTestCalls != 2 {
		t.Errorf("FindMRs() with Refresh=true made %d total SmokeTest calls, want 2", client.smokeTestCalls)
	}
}

// TestFindMRs_strictModeNeverCallsSmokeTest leaves smokeTestFn nil:
// fakeClient panics if SmokeTest is ever called, proving --strict never
// probes regardless of caching (TZ.md section 5.3c) -- search.SelectStrategy
// returns before it would ever reach cachingSmokeClient's SmokeTest.
func TestFindMRs_strictModeNeverCallsSmokeTest(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	summaries := []gitlab.MergeRequestSummary{{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7}, UserNotesCount: 10}}
	makeReq := func(moreThan int) FindMRsRequest {
		return FindMRsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  moreThan,
			Strict:    true,
			Dir:       dir,
			Now:       func() time.Time { return now },
		}
	}

	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	if _, err := FindMRs(context.Background(), client, makeReq(1)); err != nil {
		t.Fatalf("FindMRs() first strict call error = %v", err)
	}
	if _, err := FindMRs(context.Background(), client, makeReq(2)); err != nil {
		t.Fatalf("FindMRs() second strict call error = %v", err)
	}
	if client.smokeTestCalls != 0 {
		t.Errorf("FindMRs() with Strict=true made %d SmokeTest calls, want 0 always", client.smokeTestCalls)
	}
}

func TestFindMRs_fetchedAtIsAlwaysUTC(t *testing.T) {
	dir := t.TempDir()
	loc := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, time.July, 15, 23, 14, 35, 0, loc)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return []gitlab.MergeRequestSummary{
				{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7, CreatedAt: at, UpdatedAt: at}, UserNotesCount: 10},
			}, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	result, err := FindMRs(context.Background(), client, req)
	if err != nil {
		t.Fatalf("FindMRs() error = %v", err)
	}

	fetchedAt := result.Doc.Header.Source.FetchedAt
	if fetchedAt.Location() != time.UTC {
		t.Errorf("FetchedAt.Location() = %v, want time.UTC", fetchedAt.Location())
	}
	if !fetchedAt.Equal(now) {
		t.Errorf("FetchedAt = %v, want the same instant as %v", fetchedAt, now)
	}
}

func TestFindMRs_refreshForcesMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	baseReq := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	summaries := []gitlab.MergeRequestSummary{{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7}, UserNotesCount: 10}}
	makeClient := func() *fakeClient {
		return &fakeClient{
			mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				return summaries, nil
			},
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return notesFrom(42, 4, at), nil
			},
		}
	}

	if _, err := FindMRs(context.Background(), makeClient(), baseReq); err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}

	refreshReq := baseReq
	refreshReq.Cache = CacheOptions{Refresh: true}
	client2 := makeClient()
	result, err := FindMRs(context.Background(), client2, refreshReq)
	if err != nil {
		t.Fatalf("FindMRs() refresh call error = %v", err)
	}
	if result.CacheHit {
		t.Errorf("FindMRs() with Refresh=true CacheHit = true, want false")
	}
}

func TestFindMRs_expiredEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	baseReq := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Cache:     CacheOptions{TTL: time.Hour},
	}

	summaries := []gitlab.MergeRequestSummary{{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7}, UserNotesCount: 10}}
	makeClient := func() *fakeClient {
		return &fakeClient{
			mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				return summaries, nil
			},
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return notesFrom(42, 4, at), nil
			},
		}
	}

	req1 := baseReq
	req1.Now = func() time.Time { return t0 }
	if _, err := FindMRs(context.Background(), makeClient(), req1); err != nil {
		t.Fatalf("FindMRs() first call error = %v", err)
	}

	req2 := baseReq
	req2.Now = func() time.Time { return t0.Add(2 * time.Hour) }
	client2 := makeClient()
	result, err := FindMRs(context.Background(), client2, req2)
	if err != nil {
		t.Fatalf("FindMRs() expired call error = %v", err)
	}
	if result.CacheHit {
		t.Errorf("FindMRs() after TTL expiry CacheHit = true, want false")
	}
}

// TestFindMRs_differentToolVersionArtifactIsMissAndRefetches is the
// regression guard for the real incident TZ.md section 4.6 documents: a
// within-TTL mr_list artifact on disk was written under a query hash
// that folded in a DIFFERENT tool version than the one this call runs
// under (as a v0.2.0-written artifact would look to a v0.2.1 binary).
// That must be a cache miss -- FindMRs must actually call the client and
// return the fresh fetch's items, never the stale file's -- proving
// cache.QueryHash's toolVersion is load-bearing, not merely present.
func TestFindMRs_differentToolVersionArtifactIsMissAndRefetches(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Strict:    true,
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	// staleQuery mirrors exactly what FindMRs builds internally from req
	// above (same gitlab_url, resolved numeric user_id, dates, more_than,
	// no group/project/mr) -- the only difference from what FindMRs hashes
	// today is the tool version passed to cache.QueryHash.
	staleQuery := domain.Query{
		GitlabURL: "https://gitlab.example.com",
		UserID:    42,
		From:      req.From,
		To:        req.To,
		MoreThan:  req.MoreThan,
	}
	staleHash, err := cache.QueryHash(staleQuery, "not-"+ToolVersion)
	if err != nil {
		t.Fatalf("cache.QueryHash() error = %v", err)
	}
	stalePath, err := artifactPath(dir, artifact.KindMRList, staleHash, artifact.FormatYAML)
	if err != nil {
		t.Fatalf("artifactPath() error = %v", err)
	}
	staleDoc := artifact.MRList{
		Header: artifact.Header{
			Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: now.Add(-time.Minute)},
		},
		Query: staleQuery,
		Items: []artifact.MRItem{{ProjectID: 999, MRIID: 999, CommentCount: 999}},
	}
	if err := artifact.WriteMRList(staleDoc, artifact.FormatYAML, stalePath); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	calls := 0
	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7, CreatedAt: at, UpdatedAt: at}, UserNotesCount: 10},
	}
	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			calls++
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	result, err := FindMRs(context.Background(), client, req)
	if err != nil {
		t.Fatalf("FindMRs() error = %v", err)
	}
	if result.CacheHit {
		t.Fatalf("FindMRs() CacheHit = true, want false: a within-TTL artifact written under a different tool version must not count as a hit")
	}
	if calls < 1 {
		t.Errorf("FindMRs() made %d MergeRequests calls, want >= 1: it must actually fetch, not read the stale artifact", calls)
	}
	if len(result.Doc.Items) != 1 || result.Doc.Items[0].CommentCount != 4 {
		t.Fatalf("FindMRs() Items = %+v, want the fresh fetch's one item with comment_count 4, not the stale artifact's", result.Doc.Items)
	}
}

func TestFindMRs_roundTripsArtifactWithPathProjectScope(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "alice",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  2,
		Project:   "my-group/repo",
		Strict:    true,
		Dir:       dir,
		Format:    artifact.FormatJSON,
		Now:       func() time.Time { return now },
	}

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo", Title: "fix", WebURL: "https://x/y", CreatedAt: at, UpdatedAt: at}, UserNotesCount: 10},
	}
	client := &fakeClient{
		resolveUserIDFn: func(ctx context.Context, username string) (int64, error) {
			if username != "alice" {
				t.Fatalf("ResolveUserID(%q), want alice", username)
			}
			return 42, nil
		},
		projectMergeRequestsFn: func(ctx context.Context, project gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			if project.String() != "my-group/repo" {
				t.Fatalf("ProjectMergeRequests project = %q, want my-group/repo", project.String())
			}
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 3, at), nil
		},
	}

	result, err := FindMRs(context.Background(), client, req)
	if err != nil {
		t.Fatalf("FindMRs() error = %v", err)
	}
	if result.CacheHit {
		t.Fatalf("FindMRs() CacheHit = true, want false")
	}

	got, err := artifact.ReadMRList(result.Path)
	if err != nil {
		t.Fatalf("ReadMRList() error = %v", err)
	}
	if !reflect.DeepEqual(got, result.Doc) {
		t.Errorf("ReadMRList() = %+v, want %+v", got, result.Doc)
	}
	if got.Query.UserID != 42 {
		t.Errorf("Query.UserID = %d, want 42", got.Query.UserID)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly 1", got.Items)
	}
	item := got.Items[0]
	if item.CommentCount != 3 || item.ProjectPath != "my-group/repo" || item.Title != "fix" || item.WebURL != "https://x/y" {
		t.Errorf("Items[0] = %+v, unexpected", item)
	}
}

func TestFindMRs_pointModeRequiresProject(t *testing.T) {
	_, err := FindMRs(context.Background(), &fakeClient{}, FindMRsRequest{User: "42", MR: int64Ptr(9)})
	if !errors.Is(err, ErrPointModeRequiresProject) {
		t.Errorf("error = %v, want ErrPointModeRequiresProject", err)
	}
}

func TestFindMRs_pointModeMakesNoSearchStrategyCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  3,
		Project:   "my-group/repo",
		MR:        int64Ptr(9),
		Dir:       dir,
		Now:       func() time.Time { return now },
	}

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo", Title: "fix", WebURL: "https://x/y", CreatedAt: at, UpdatedAt: at}},
	}
	// commentEventsFn, smokeTestFn, mergeRequestsFn, groupMergeRequestsFn,
	// projectMergeRequestsFn, getProjectFn and groupProjectsFn are all
	// left nil: fakeClient panics if point mode calls any candidate
	// search method, proving point mode runs no discovery of any kind.
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			if project.String() != "my-group/repo" || len(iids) != 1 || iids[0] != 9 {
				t.Fatalf("ProjectMergeRequestsByIIDs(%s, %v), want (my-group/repo, [9])", project.String(), iids)
			}
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 4, at), nil
		},
	}

	result, err := FindMRs(context.Background(), client, req)
	if err != nil {
		t.Fatalf("FindMRs() error = %v", err)
	}
	if client.resolveUserIDCalls != 0 {
		t.Errorf("FindMRs() with numeric user called ResolveUserID %d times, want 0", client.resolveUserIDCalls)
	}
	if len(result.Doc.Items) != 1 || result.Doc.Items[0].CommentCount != 4 {
		t.Fatalf("Items = %+v, want one item with comment_count 4", result.Doc.Items)
	}
}

func TestFindMRs_pointModeBoundary(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	summaries := []gitlab.MergeRequestSummary{{MergeRequest: domain.MergeRequest{ProjectID: 5, IID: 9}}}

	makeReq := func(dir string) FindMRsRequest {
		return FindMRsRequest{
			GitlabURL: "https://gitlab.example.com",
			User:      "42",
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  5,
			Project:   "my-group/repo",
			MR:        int64Ptr(9),
			Dir:       dir,
			Now:       func() time.Time { return now },
		}
	}

	t.Run("exactly more_than is excluded", func(t *testing.T) {
		dir := t.TempDir()
		client := &fakeClient{
			projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
				return summaries, nil
			},
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return notesFrom(42, 5, at), nil
			},
		}
		result, err := FindMRs(context.Background(), client, makeReq(dir))
		if err != nil {
			t.Fatalf("FindMRs() error = %v", err)
		}
		if len(result.Doc.Items) != 0 {
			t.Errorf("Items = %+v, want empty (exactly more_than must be excluded)", result.Doc.Items)
		}
	})

	t.Run("more_than plus one is included", func(t *testing.T) {
		dir := t.TempDir()
		client := &fakeClient{
			projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
				return summaries, nil
			},
			discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
				return notesFrom(42, 6, at), nil
			},
		}
		result, err := FindMRs(context.Background(), client, makeReq(dir))
		if err != nil {
			t.Fatalf("FindMRs() error = %v", err)
		}
		if len(result.Doc.Items) != 1 || result.Doc.Items[0].CommentCount != 6 {
			t.Fatalf("Items = %+v, want exactly one item with comment_count 6", result.Doc.Items)
		}
	})
}

func TestFindMRs_pointModeArtifactShapeMatchesConverterOutput(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	mr := domain.MergeRequest{ProjectID: 5, IID: 9, ProjectPath: "my-group/repo", Title: "fix", WebURL: "https://x/y", CreatedAt: at, UpdatedAt: at}
	summaries := []gitlab.MergeRequestSummary{{MergeRequest: mr}}

	req := FindMRsRequest{
		GitlabURL: "https://gitlab.example.com",
		User:      "42",
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  2,
		Project:   "my-group/repo",
		MR:        int64Ptr(9),
		Dir:       dir,
		Now:       func() time.Time { return now },
	}
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return notesFrom(42, 3, at), nil
		},
	}

	result, err := FindMRs(context.Background(), client, req)
	if err != nil {
		t.Fatalf("FindMRs() error = %v", err)
	}
	if len(result.Doc.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly 1", result.Doc.Items)
	}

	// The item any other strategy's toMRList conversion would have
	// produced for the exact same underlying domain.MergeRequest data,
	// through the exact same converter -- proves point mode's output has
	// the identical shape (TZ.md section 14, item 11's cross-strategy
	// shape parity, now also true for the point-mode path).
	mrWithCount := mr
	mrWithCount.CommentCount = 3
	want := toMRList(artifact.Header{}, domain.Query{}, search.Result{Items: []domain.MergeRequest{mrWithCount}}).Items[0]

	if result.Doc.Items[0] != want {
		t.Errorf("point mode item = %+v, want %+v (same shape the converter produces for any other strategy)", result.Doc.Items[0], want)
	}
}
