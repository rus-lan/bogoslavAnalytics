package app

import (
	"context"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

// fakeClient implements FindMRsClient in-memory (TZ.md section 2.4),
// mirroring the fakeClient pattern search's own tests use: every method
// panics if its function field is nil, so a test that does not expect a
// call to fire finds out immediately instead of silently getting zero
// values.
type fakeClient struct {
	commentEventsFn              func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error)
	discussionsFn                func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error)
	getProjectFn                 func(ctx context.Context, project gitlab.ID) (domain.Project, error)
	groupProjectsFn              func(ctx context.Context, group gitlab.ID) ([]domain.Project, error)
	mergeRequestsFn              func(ctx context.Context, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	groupMergeRequestsFn         func(ctx context.Context, group gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	projectMergeRequestsFn       func(ctx context.Context, project gitlab.ID, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	projectMergeRequestsByIIDsFn func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error)
	smokeTestFn                  func(ctx context.Context, userID int64) (domain.SmokeResult, error)
	smokeTestCalls               int
	resolveUserIDFn              func(ctx context.Context, username string) (int64, error)
	resolveUserIDCalls           int
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
	f.smokeTestCalls++
	if f.smokeTestFn == nil {
		panic("fakeClient: SmokeTest called but not configured")
	}
	return f.smokeTestFn(ctx, userID)
}

func (f *fakeClient) ResolveUserID(ctx context.Context, username string) (int64, error) {
	f.resolveUserIDCalls++
	if f.resolveUserIDFn == nil {
		panic("fakeClient: ResolveUserID called but not configured")
	}
	return f.resolveUserIDFn(ctx, username)
}

var _ FindMRsClient = (*fakeClient)(nil)

// fakeDiscussionsClient implements GetCommentsClient in-memory for
// GetComments tests.
type fakeDiscussionsClient struct {
	discussionsFn      func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error)
	calls              int
	resolveUserIDFn    func(ctx context.Context, username string) (int64, error)
	resolveUserIDCalls int
}

func (f *fakeDiscussionsClient) Discussions(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
	f.calls++
	if f.discussionsFn == nil {
		panic("fakeDiscussionsClient: Discussions called but not configured")
	}
	return f.discussionsFn(ctx, project, mrIID)
}

func (f *fakeDiscussionsClient) ResolveUserID(ctx context.Context, username string) (int64, error) {
	f.resolveUserIDCalls++
	if f.resolveUserIDFn == nil {
		panic("fakeDiscussionsClient: ResolveUserID called but not configured")
	}
	return f.resolveUserIDFn(ctx, username)
}

var _ GetCommentsClient = (*fakeDiscussionsClient)(nil)

// note builds a domain.Note with the fields this package's tests care
// about, mirroring the search package's own test helper of the same
// name.
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

// notesFrom builds n notes from userID, all inside a fixed instant, each
// wrapped in its own single-note discussion.
func notesFrom(userID int64, n int, at time.Time) []domain.Discussion {
	var out []domain.Discussion
	for i := range n {
		out = append(out, discussion("d", note(int64(i)+1, userID, false, at)))
	}
	return out
}
