package search

import (
	"context"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

func TestBruteforceCandidates_windowPredicateNeverIncludesUpdatedBefore(t *testing.T) {
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{
		UserID:   42,
		Range:    mustDateRange(from, to),
		MoreThan: 0,
	}

	var got gitlab.MergeRequestWindow
	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			got = w
			return nil, nil
		},
	}

	if _, err := bruteforceCandidates(context.Background(), client, p); err != nil {
		t.Fatalf("bruteforceCandidates() error = %v", err)
	}

	// gitlab.MergeRequestWindow only ever has CreatedBefore and
	// UpdatedAfter fields -- there is structurally no updated_before to
	// send. This assertion pins the values search actually builds.
	if got.CreatedBefore != to {
		t.Errorf("window.CreatedBefore = %s, want %s (= Range.To)", got.CreatedBefore, to)
	}
	if got.UpdatedAfter != from {
		t.Errorf("window.UpdatedAfter = %s, want %s (= Range.From)", got.UpdatedAfter, from)
	}
}

func TestBruteforceCandidates_preFilterSkipsUserNotesCountBelowMoreThan(t *testing.T) {
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{
		UserID:   42,
		Range:    mustDateRange(from, to),
		MoreThan: 5,
	}

	items := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 1}, UserNotesCount: 4},
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 2}, UserNotesCount: 5},
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 3}, UserNotesCount: 6},
	}
	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return items, nil
		},
	}

	got, err := bruteforceCandidates(context.Background(), client, p)
	if err != nil {
		t.Fatalf("bruteforceCandidates() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("bruteforceCandidates() = %+v, want 2 candidates (mr 2 and mr 3, user_notes_count >= 5)", got)
	}
	for _, cand := range got {
		if cand.IID == 1 {
			t.Errorf("candidate %+v has user_notes_count 4 < more_than 5, must have been pre-filtered out", cand)
		}
	}
}

func TestMRListerFor_selectsProjectGroupOrGlobalByScope(t *testing.T) {
	projectID := gitlab.NumericID(11)
	groupID := gitlab.NumericID(22)
	window := gitlab.MergeRequestWindow{}

	t.Run("project scope", func(t *testing.T) {
		var gotProjectID gitlab.ID
		client := &fakeClient{
			projectMergeRequestsFn: func(ctx context.Context, id gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				gotProjectID = id
				return nil, nil
			},
		}
		list := mrListerFor(client, Scope{ProjectID: &projectID})
		if _, err := list(context.Background(), window); err != nil {
			t.Fatalf("list() error = %v", err)
		}
		if gotProjectID != projectID {
			t.Errorf("ProjectMergeRequests called with %s, want %s", gotProjectID, projectID)
		}
	})

	t.Run("group scope", func(t *testing.T) {
		var gotGroupID gitlab.ID
		client := &fakeClient{
			groupMergeRequestsFn: func(ctx context.Context, id gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				gotGroupID = id
				return nil, nil
			},
		}
		list := mrListerFor(client, Scope{GroupID: &groupID})
		if _, err := list(context.Background(), window); err != nil {
			t.Fatalf("list() error = %v", err)
		}
		if gotGroupID != groupID {
			t.Errorf("GroupMergeRequests called with %s, want %s", gotGroupID, groupID)
		}
	})

	t.Run("no scope: global", func(t *testing.T) {
		called := false
		client := &fakeClient{
			mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				called = true
				return nil, nil
			},
		}
		list := mrListerFor(client, Scope{})
		if _, err := list(context.Background(), window); err != nil {
			t.Fatalf("list() error = %v", err)
		}
		if !called {
			t.Error("MergeRequests was not called for the global scope")
		}
	})
}

// TestMRListerFor_pathScopeGoesStraightThroughWithoutLookup proves the
// benefit TZ.md section 14, item 1 now resolves for bruteforce: a
// namespaced project or group path goes straight into the corresponding
// :id path parameter, with no separate resolution call first. fakeClient's
// groupProjectsFn is intentionally left nil, so any attempt to resolve the
// path via a lookup call would panic instead of silently succeeding.
func TestMRListerFor_pathScopeGoesStraightThroughWithoutLookup(t *testing.T) {
	window := gitlab.MergeRequestWindow{}

	t.Run("project path scope", func(t *testing.T) {
		project := gitlab.PathID("my-group/my-project")
		var gotProject gitlab.ID
		client := &fakeClient{
			projectMergeRequestsFn: func(ctx context.Context, id gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				gotProject = id
				return nil, nil
			},
		}
		list := mrListerFor(client, Scope{ProjectID: &project})
		if _, err := list(context.Background(), window); err != nil {
			t.Fatalf("list() error = %v", err)
		}
		if gotProject != project {
			t.Errorf("ProjectMergeRequests called with %s, want %s (path passed straight through)", gotProject, project)
		}
		if client.getProjectCalls != 0 {
			t.Errorf("GetProject() called %d times, want 0 (bruteforce needs no path resolution)", client.getProjectCalls)
		}
	})

	t.Run("group path scope", func(t *testing.T) {
		group := gitlab.PathID("my-group/subgroup")
		var gotGroup gitlab.ID
		client := &fakeClient{
			groupMergeRequestsFn: func(ctx context.Context, id gitlab.ID, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
				gotGroup = id
				return nil, nil
			},
		}
		list := mrListerFor(client, Scope{GroupID: &group})
		if _, err := list(context.Background(), window); err != nil {
			t.Fatalf("list() error = %v", err)
		}
		if gotGroup != group {
			t.Errorf("GroupMergeRequests called with %s, want %s (path passed straight through)", gotGroup, group)
		}
	})
}

func TestDedupeMergeRequests_removesDuplicateProjectIDAndIID(t *testing.T) {
	items := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 1}},
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 1}},
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 2}},
		{MergeRequest: domain.MergeRequest{ProjectID: 2, IID: 1}},
	}
	got := dedupeMergeRequests(items)
	if len(got) != 3 {
		t.Fatalf("dedupeMergeRequests() = %+v, want 3 unique (project_id, iid) entries", got)
	}
}
