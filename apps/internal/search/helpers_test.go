package search

import (
	"context"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// fakeClient is an in-memory search.Client used by every test in this
// package (TZ.md section 2.4: search's own consumer-side interface must be
// testable without a real GitLab instance). Every method panics if its
// function field is nil, so a test that does not expect a call to fire
// finds out immediately instead of silently getting zero values.
type fakeClient struct {
	commentEventsFn              func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error)
	discussionsFn                func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error)
	getProjectFn                 func(ctx context.Context, project gitlab.ID) (domain.Project, error)
	getProjectCalls              int
	groupProjectsFn              func(ctx context.Context, group gitlab.ID) ([]domain.Project, error)
	mergeRequestsFn              func(ctx context.Context, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	groupMergeRequestsFn         func(ctx context.Context, group gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	projectMergeRequestsFn       func(ctx context.Context, project gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	projectMergeRequestsByIIDsFn func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error)
	smokeTestFn                  func(ctx context.Context, userID int64) (domain.SmokeResult, error)
}

func (f *fakeClient) CommentEvents(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
	if f.commentEventsFn == nil {
		panic("fakeClient: CommentEvents called but not configured")
	}
	return f.commentEventsFn(ctx, userID, window)
}

func (f *fakeClient) Discussions(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
	if f.discussionsFn == nil {
		panic("fakeClient: Discussions called but not configured")
	}
	return f.discussionsFn(ctx, project, mrIID)
}

func (f *fakeClient) GetProject(ctx context.Context, project gitlab.ID) (domain.Project, error) {
	f.getProjectCalls++
	if f.getProjectFn == nil {
		panic("fakeClient: GetProject called but not configured")
	}
	return f.getProjectFn(ctx, project)
}

func (f *fakeClient) GroupProjects(ctx context.Context, group gitlab.ID) ([]domain.Project, error) {
	if f.groupProjectsFn == nil {
		panic("fakeClient: GroupProjects called but not configured")
	}
	return f.groupProjectsFn(ctx, group)
}

func (f *fakeClient) MergeRequests(ctx context.Context, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
	if f.mergeRequestsFn == nil {
		panic("fakeClient: MergeRequests called but not configured")
	}
	return f.mergeRequestsFn(ctx, window)
}

func (f *fakeClient) GroupMergeRequests(ctx context.Context, group gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
	if f.groupMergeRequestsFn == nil {
		panic("fakeClient: GroupMergeRequests called but not configured")
	}
	return f.groupMergeRequestsFn(ctx, group, window)
}

func (f *fakeClient) ProjectMergeRequests(ctx context.Context, project gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
	if f.projectMergeRequestsFn == nil {
		panic("fakeClient: ProjectMergeRequests called but not configured")
	}
	return f.projectMergeRequestsFn(ctx, project, window)
}

func (f *fakeClient) ProjectMergeRequestsByIIDs(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
	if f.projectMergeRequestsByIIDsFn == nil {
		panic("fakeClient: ProjectMergeRequestsByIIDs called but not configured")
	}
	return f.projectMergeRequestsByIIDsFn(ctx, project, iids)
}

func (f *fakeClient) SmokeTest(ctx context.Context, userID int64) (domain.SmokeResult, error) {
	if f.smokeTestFn == nil {
		panic("fakeClient: SmokeTest called but not configured")
	}
	return f.smokeTestFn(ctx, userID)
}

var _ Client = (*fakeClient)(nil)

// mustDateRange builds a domain.DateRange, failing fast (panic, since this
// only ever runs with fixed test literals) if from is after to.
func mustDateRange(from, to domain.Date) domain.DateRange {
	r, err := domain.NewDateRange(from, to)
	if err != nil {
		panic(err)
	}
	return r
}

// note builds a domain.Note with the fields CountComments and the events
// candidate mapping care about.
func note(id, authorID int64, system bool, createdAt time.Time) domain.Note {
	return domain.Note{
		ID:           id,
		Type:         domain.NoteTypeDiscussion,
		Body:         "body",
		Author:       domain.Author{ID: authorID},
		CreatedAt:    createdAt,
		System:       system,
		NoteableType: "MergeRequest",
	}
}

// discussion wraps notes into a single-thread domain.Discussion.
func discussion(id string, notes ...domain.Note) domain.Discussion {
	return domain.Discussion{ID: id, Notes: notes}
}

// commentEvent builds a gitlab.CommentEvent for a merge request comment.
func commentEvent(projectID, mrIID int64, system bool, createdAt time.Time) gitlab.CommentEvent {
	return gitlab.CommentEvent{
		ProjectID:  projectID,
		ActionName: "commented on",
		TargetType: "DiscussionNote",
		CreatedAt:  createdAt,
		Note: gitlab.EventNote{
			System:       system,
			NoteableType: "MergeRequest",
			NoteableIID:  mrIID,
		},
	}
}
