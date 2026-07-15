package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
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
		UserID:       42,
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
		UserID: 42,
		From:   domain.NewDate(2026, time.January, 1),
		To:     domain.NewDate(2026, time.June, 30),
		MRs:    []artifact.MRRef{{ProjectID: 1, MRIID: 7}, {ProjectID: 2, MRIID: 8}},
		Dir:    dir,
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
		UserID:    42,
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
		UserID: 42,
		From:   domain.NewDate(2026, time.January, 1),
		To:     domain.NewDate(2026, time.June, 30),
	})
	if !errors.Is(err, ErrNoMergeRequests) {
		t.Errorf("error = %v, want ErrNoMergeRequests", err)
	}
}

func TestGetComments_ambiguousInputIsError(t *testing.T) {
	_, err := GetComments(context.Background(), &fakeDiscussionsClient{}, GetCommentsRequest{
		UserID:       42,
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
		UserID:    42,
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
		UserID:    42,
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
		UserID:    42,
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
