package search

import (
	"context"
	"errors"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

func TestEnrichEventsCandidates_oneCallPerProjectNotPerMR(t *testing.T) {
	items := []domain.MergeRequest{
		{ProjectID: 1, IID: 10, CommentCount: 4},
		{ProjectID: 1, IID: 11, CommentCount: 5},
		{ProjectID: 2, IID: 20, CommentCount: 6},
	}

	callsByProject := make(map[int64]int)
	iidsByProject := make(map[int64][]int64)
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			id, ok := scopeProjectNumericID(project)
			if !ok {
				t.Fatalf("ProjectMergeRequestsByIIDs called with non-numeric project %s", project)
			}
			callsByProject[id]++
			iidsByProject[id] = append([]int64(nil), iids...)

			out := make([]gitlab.MergeRequestSummary, len(iids))
			for i, iid := range iids {
				out[i] = gitlab.MergeRequestSummary{MergeRequest: domain.MergeRequest{ProjectID: id, IID: iid, Title: "t"}}
			}
			return out, nil
		},
	}

	got, err := enrichEventsCandidates(context.Background(), client, items)
	if err != nil {
		t.Fatalf("enrichEventsCandidates() error = %v", err)
	}
	if len(got) != len(items) {
		t.Fatalf("enrichEventsCandidates() returned %d items, want %d", len(got), len(items))
	}

	if callsByProject[1] != 1 {
		t.Errorf("ProjectMergeRequestsByIIDs called %d times for project 1, want exactly 1 (batched, not one call per merge request)", callsByProject[1])
	}
	if callsByProject[2] != 1 {
		t.Errorf("ProjectMergeRequestsByIIDs called %d times for project 2, want exactly 1", callsByProject[2])
	}
	if len(iidsByProject[1]) != 2 {
		t.Errorf("project 1's single call carried %d iids, want both (10 and 11) batched into it", len(iidsByProject[1]))
	}
}

func TestEnrichEventsCandidates_missingEnrichmentKeepsExistingFields(t *testing.T) {
	items := []domain.MergeRequest{
		{ProjectID: 1, IID: 10, CommentCount: 4},
	}
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			// The merge request was deleted (or is otherwise no longer
			// listable) after the comment was made: the batched fetch
			// comes back empty for it.
			return nil, nil
		},
	}

	got, err := enrichEventsCandidates(context.Background(), client, items)
	if err != nil {
		t.Fatalf("enrichEventsCandidates() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("enrichEventsCandidates() returned %d items, want 1 (kept, not dropped)", len(got))
	}
	if got[0].ProjectID != 1 || got[0].IID != 10 || got[0].CommentCount != 4 {
		t.Errorf("enrichEventsCandidates() = %+v, want ProjectID=1 IID=10 CommentCount=4 preserved", got[0])
	}
	if got[0].Title != "" || got[0].WebURL != "" {
		t.Errorf("enrichEventsCandidates() = %+v, want Title/WebURL still zero (nothing to fill them in from)", got[0])
	}
}

func TestEnrichEventsCandidates_noItemsMakesNoCall(t *testing.T) {
	called := false
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			called = true
			return nil, nil
		},
	}
	got, err := enrichEventsCandidates(context.Background(), client, nil)
	if err != nil {
		t.Fatalf("enrichEventsCandidates() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("enrichEventsCandidates() = %+v, want empty", got)
	}
	if called {
		t.Error("ProjectMergeRequestsByIIDs called with zero candidates, want no call at all")
	}
}

func TestEnrichEventsCandidates_propagatesError(t *testing.T) {
	items := []domain.MergeRequest{{ProjectID: 1, IID: 10}}
	wantErr := gitlab.ErrRateLimited
	client := &fakeClient{
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			return nil, wantErr
		},
	}
	if _, err := enrichEventsCandidates(context.Background(), client, items); !errors.Is(err, wantErr) {
		t.Fatalf("enrichEventsCandidates() error = %v, want wrapping %v", err, wantErr)
	}
}
