package search

import (
	"context"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
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
	projectID := int64(99)
	set, err := scopeProjectSet(context.Background(), &fakeClient{}, Scope{ProjectID: &projectID})
	if err != nil {
		t.Fatalf("scopeProjectSet() error = %v", err)
	}
	if len(set) != 1 || !set[99] {
		t.Errorf("scopeProjectSet() = %v, want {99: true}", set)
	}
}

func TestScopeProjectSet_restrictsToGroupProjects(t *testing.T) {
	groupID := int64(7)
	client := &fakeClient{
		groupProjectsFn: func(ctx context.Context, gotGroupID int64) ([]domain.Project, error) {
			if gotGroupID != groupID {
				t.Errorf("GroupProjects() called with group %d, want %d", gotGroupID, groupID)
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

	groupID := int64(7)
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
		groupProjectsFn: func(ctx context.Context, gotGroupID int64) ([]domain.Project, error) {
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
