package search

import (
	"context"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

func TestEventsCandidates_dropsSystemNotesAndNonMergeRequestNoteables(t *testing.T) {
	createdAt := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	events := []gitlab.CommentEvent{
		commentEvent(1, 10, false, createdAt),
		commentEvent(1, 10, false, createdAt.Add(time.Minute)),
		// System note on a merge request: must be dropped.
		commentEvent(1, 11, true, createdAt),
		// Non-MergeRequest noteable (e.g. an issue comment): must be
		// dropped.
		{
			ProjectID:  1,
			ActionName: "commented on",
			TargetType: "Note",
			CreatedAt:  createdAt,
			Note: gitlab.EventNote{
				System:       false,
				NoteableType: "Issue",
				NoteableIID:  12,
			},
		},
	}

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   42,
		Range:    mustDateRange(from, to),
		MoreThan: 0,
	}

	client := &fakeClient{
		commentEventsFn: func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
	}

	got, err := eventsCandidates(context.Background(), client, p)
	if err != nil {
		t.Fatalf("eventsCandidates() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("eventsCandidates() = %+v, want exactly 1 candidate (project 1, mr 10)", got)
	}
	if got[0].ProjectID != 1 || got[0].IID != 10 {
		t.Errorf("candidate = %+v, want project_id=1 iid=10", got[0])
	}
}

func TestEventsCandidates_preliminaryCountIsSupersetAtGreaterOrEqualThreshold(t *testing.T) {
	createdAt := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	events := []gitlab.CommentEvent{
		// MR 10: exactly 2 events.
		commentEvent(1, 10, false, createdAt),
		commentEvent(1, 10, false, createdAt.Add(time.Minute)),
		// MR 20: exactly 1 event.
		commentEvent(1, 20, false, createdAt),
	}

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   42,
		Range:    mustDateRange(from, to),
		MoreThan: 2,
	}
	client := &fakeClient{
		commentEventsFn: func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
	}

	got, err := eventsCandidates(context.Background(), client, p)
	if err != nil {
		t.Fatalf("eventsCandidates() error = %v", err)
	}
	if len(got) != 1 || got[0].IID != 10 {
		t.Fatalf("eventsCandidates() = %+v, want only mr 10 (preliminary count 2 >= more_than 2)", got)
	}
}

func TestScopeProjectSet_restrictsToSingleProject(t *testing.T) {
	projectID := gitlab.NumericID(99)
	set, err := scopeProjectSet(context.Background(), &fakeClient{}, Scope{ProjectID: &projectID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 1 || !set[99] {
		t.Errorf("scopeProjectSet() = %v, want {99: true}", set)
	}
}

func TestScopeProjectSet_pathProjectScopeResolvesViaGetProject(t *testing.T) {
	// A namespaced path has no numeric project id to compare against
	// comment events' always-numeric project_id field (TZ.md section 5.1.2):
	// scopeProjectSet resolves it via client.GetProject instead of failing.
	projectID := gitlab.PathID("my-group/my-project")
	client := &fakeClient{
		getProjectFn: func(ctx context.Context, gotProjectID gitlab.ID) (domain.Project, error) {
			if gotProjectID != projectID {
				t.Errorf("GetProject() called with project %s, want %s", gotProjectID, projectID)
			}
			return domain.Project{ID: 99, Path: "my-group/my-project"}, nil
		},
	}
	set, err := scopeProjectSet(context.Background(), client, Scope{ProjectID: &projectID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 1 || !set[99] {
		t.Errorf("scopeProjectSet() = %v, want {99: true}", set)
	}
	if client.getProjectCalls != 1 {
		t.Errorf("GetProject() called %d times, want exactly 1", client.getProjectCalls)
	}
}

func TestScopeProjectSet_numericProjectScopeMakesNoGetProjectCall(t *testing.T) {
	projectID := gitlab.NumericID(99)
	client := &fakeClient{}
	set, err := scopeProjectSet(context.Background(), client, Scope{ProjectID: &projectID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 1 || !set[99] {
		t.Errorf("scopeProjectSet() = %v, want {99: true}", set)
	}
	if client.getProjectCalls != 0 {
		t.Errorf("GetProject() called %d times, want 0 (numeric scope needs no lookup)", client.getProjectCalls)
	}
}

func TestScopeProjectSet_restrictsToGroupProjects(t *testing.T) {
	groupID := gitlab.NumericID(7)
	client := &fakeClient{
		groupProjectsFn: func(ctx context.Context, gotGroupID gitlab.ID) ([]domain.Project, error) {
			if gotGroupID != groupID {
				t.Errorf("GroupProjects() called with group %s, want %s", gotGroupID, groupID)
			}
			return []domain.Project{{ID: 1}, {ID: 2}}, nil
		},
	}
	set, err := scopeProjectSet(context.Background(), client, Scope{GroupID: &groupID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 2 || !set[1] || !set[2] {
		t.Errorf("scopeProjectSet() = %v, want {1: true, 2: true}", set)
	}
}

func TestScopeProjectSet_groupPathScopeNeedsNoResolution(t *testing.T) {
	// A namespaced group path goes straight into GroupProjects's :id
	// parameter (TZ.md section 14, item 1, now resolved): the numeric
	// project ids scopeProjectSet builds always come back from the API
	// response itself, so a group scope never needs the kind of
	// client-side numeric lookup a single project scope does.
	groupID := gitlab.PathID("my-group/subgroup")
	client := &fakeClient{
		groupProjectsFn: func(ctx context.Context, gotGroupID gitlab.ID) ([]domain.Project, error) {
			if gotGroupID != groupID {
				t.Errorf("GroupProjects() called with group %s, want %s", gotGroupID, groupID)
			}
			return []domain.Project{{ID: 5, Path: "my-group/subgroup/repo"}}, nil
		},
	}
	set, err := scopeProjectSet(context.Background(), client, Scope{GroupID: &groupID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 1 || !set[5] {
		t.Errorf("scopeProjectSet() = %v, want {5: true}", set)
	}
	if client.getProjectCalls != 0 {
		t.Errorf("GetProject() called %d times, want 0 (a group scope resolves via GroupProjects, never GetProject)", client.getProjectCalls)
	}
}

func TestScopeProjectSet_noRestrictionWhenScopeEmpty(t *testing.T) {
	set, err := scopeProjectSet(context.Background(), &fakeClient{}, Scope{})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if set != nil {
		t.Errorf("scopeProjectSet() = %v, want nil (no restriction)", set)
	}
}

func TestEventsCandidates_groupScopeDropsEventsFromOtherProjects(t *testing.T) {
	createdAt := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	events := []gitlab.CommentEvent{
		commentEvent(1, 10, false, createdAt), // in group
		commentEvent(9, 90, false, createdAt), // not in group
	}

	groupID := gitlab.NumericID(7)
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   42,
		Range:    mustDateRange(from, to),
		MoreThan: 0,
		Scope:    Scope{GroupID: &groupID},
	}
	client := &fakeClient{
		commentEventsFn: func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
		groupProjectsFn: func(ctx context.Context, gotGroupID gitlab.ID) ([]domain.Project, error) {
			return []domain.Project{{ID: 1}}, nil
		},
	}

	got, err := eventsCandidates(context.Background(), client, p)
	if err != nil {
		t.Fatalf("eventsCandidates() error = %v", err)
	}
	if len(got) != 1 || got[0].ProjectID != 1 {
		t.Fatalf("eventsCandidates() = %+v, want only project 1's merge request", got)
	}
}
