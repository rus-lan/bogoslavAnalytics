package search

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// notesFrom builds n notes from userID, all inside a fixed instant, each
// wrapped in its own single-note discussion.
func notesFrom(userID int64, n int, at time.Time) []domain.Discussion {
	var out []domain.Discussion
	for i := range n {
		out = append(out, discussion("d", note(int64(i)+1, userID, false, at)))
	}
	return out
}

func TestFind_userWithExactlyMoreThanCommentsIsExcludedNPlusOneIsIncluded(t *testing.T) {
	const userID = 42
	const moreThan = 5
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   userID,
		Range:    mustDateRange(from, to),
		MoreThan: moreThan,
	}
	opts := Options{Strict: true} // bypasses the smoke test and the retention check

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 1}, UserNotesCount: 10},
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 2}, UserNotesCount: 10},
	}
	discussionsByMR := map[[2]int64][]domain.Discussion{
		{1, 1}: notesFrom(userID, moreThan, at),   // exactly N=5 comments
		{1, 2}: notesFrom(userID, moreThan+1, at), // N+1=6 comments
	}

	client := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return summaries, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			projectID, ok := scopeProjectNumericID(project)
			if !ok {
				t.Fatalf("Discussions() called with non-numeric project %s", project)
			}
			return discussionsByMR[[2]int64{projectID, mrIID}], nil
		},
	}

	result, err := Find(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if result.Strategy != domain.StrategyBruteforce {
		t.Errorf("Find() strategy = %q, want %q", result.Strategy, domain.StrategyBruteforce)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Find() returned %d items, want exactly 1 -- items=%+v", len(result.Items), result.Items)
	}
	got := result.Items[0]
	if got.ProjectID != 1 || got.IID != 2 {
		t.Errorf("Find() kept project=%d iid=%d, want project=1 iid=2 (the N+1 merge request)", got.ProjectID, got.IID)
	}
	if got.CommentCount != moreThan+1 {
		t.Errorf("Find() comment_count = %d, want %d", got.CommentCount, moreThan+1)
	}
	for _, it := range result.Items {
		if it.IID == 1 {
			t.Errorf("Find() kept mr 1, which has exactly more_than (%d) comments and must be excluded (boundary is strictly >)", moreThan)
		}
	}
}

func TestFind_eventsStrategyEndToEnd(t *testing.T) {
	const userID = 42
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   userID,
		Range:    mustDateRange(from, to),
		MoreThan: 1,
	}
	opts := Options{}

	events := []gitlab.CommentEvent{
		commentEvent(1, 77, false, at),
		commentEvent(1, 77, false, at.Add(time.Minute)),
	}
	discussions := []domain.Discussion{
		discussion("d", note(1, userID, false, at), note(2, userID, false, at.Add(time.Minute))),
	}
	const wantTitle = "fix bug"
	const wantWebURL = "https://gitlab.example.com/my-group/repo/-/merge_requests/77"

	var byIIDsCalls int
	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, id int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
		commentEventsFn: func(ctx context.Context, id int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return discussions, nil
		},
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			byIIDsCalls++
			return []gitlab.MergeRequestSummary{
				{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 77, Title: wantTitle, WebURL: wantWebURL}},
			}, nil
		},
	}

	result, err := Find(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if result.Strategy != domain.StrategyEvents {
		t.Errorf("Find() strategy = %q, want %q", result.Strategy, domain.StrategyEvents)
	}
	if result.Smoke != domain.SmokePassed {
		t.Errorf("Find() smoke = %q, want %q", result.Smoke, domain.SmokePassed)
	}
	if len(result.Items) != 1 {
		t.Fatalf("Find() returned %d items, want 1 -- items=%+v", len(result.Items), result.Items)
	}
	got := result.Items[0]
	if got.CommentCount != 2 {
		t.Errorf("Find() comment_count = %d, want 2", got.CommentCount)
	}
	if got.Title != wantTitle || got.WebURL != wantWebURL {
		t.Errorf("Find() = %+v, want enriched Title=%q WebURL=%q", got, wantTitle, wantWebURL)
	}
	if byIIDsCalls != 1 {
		t.Errorf("ProjectMergeRequestsByIIDs called %d times, want exactly 1", byIIDsCalls)
	}
}

// TestFind_eventsAndBruteforceStrategiesProduceEquivalentMergeRequests is
// the regression guard TZ.md section 14, item 11 asks for: the same query
// against the same fixture must produce the same domain.MergeRequest shape
// regardless of which strategy the autoselector happened to pick. Before
// enrichEventsCandidates existed, the events branch below would return
// bruteforceResult.Items[0] with only ProjectID/IID/CommentCount populated
// -- everything else zero -- which fails the reflect.DeepEqual comparison
// at the end. Reverting enrichEventsCandidates (or its call in Find) makes
// this test fail again.
func TestFind_eventsAndBruteforceStrategiesProduceEquivalentMergeRequests(t *testing.T) {
	const userID = 7
	const moreThan = 3
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.March, 20, 9, 0, 0, 0, time.UTC)

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{
		UserID:   userID,
		Range:    mustDateRange(from, to),
		MoreThan: moreThan,
	}

	fullMR := domain.MergeRequest{
		ProjectID:   5,
		ProjectPath: "my-group/repo",
		IID:         42,
		Title:       "Add feature",
		WebURL:      "https://gitlab.example.com/my-group/repo/-/merge_requests/42",
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		References:  domain.References{Full: "my-group/repo!42"},
	}
	// Hand-computed exact count: 4 notes from userID, all inside range,
	// strictly greater than moreThan (3) -- survives resolveCandidates in
	// both strategies.
	discussions := []domain.Discussion{
		discussion("d",
			note(1, userID, false, at),
			note(2, userID, false, at.Add(time.Minute)),
			note(3, userID, false, at.Add(2*time.Minute)),
			note(4, userID, false, at.Add(3*time.Minute)),
		),
	}
	discussionsFn := func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
		return discussions, nil
	}

	bruteforceClient := &fakeClient{
		mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return []gitlab.MergeRequestSummary{{MergeRequest: fullMR, UserNotesCount: 10}}, nil
		},
		discussionsFn: discussionsFn,
	}
	bruteforceResult, err := Find(context.Background(), bruteforceClient, p, Options{Strict: true})
	if err != nil {
		t.Fatalf("Find() (bruteforce) error = %v", err)
	}
	if len(bruteforceResult.Items) != 1 {
		t.Fatalf("Find() (bruteforce) returned %d items, want 1 -- items=%+v", len(bruteforceResult.Items), bruteforceResult.Items)
	}

	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	var byIIDsCalls int
	eventsClient := &fakeClient{
		smokeTestFn: func(ctx context.Context, id int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
		commentEventsFn: func(ctx context.Context, id int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return []gitlab.CommentEvent{
				commentEvent(5, 42, false, at),
				commentEvent(5, 42, false, at.Add(time.Minute)),
				commentEvent(5, 42, false, at.Add(2*time.Minute)),
			}, nil
		},
		discussionsFn: discussionsFn,
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			byIIDsCalls++
			if project != gitlab.NumericID(5) {
				t.Errorf("ProjectMergeRequestsByIIDs called with project %s, want %s", project, gitlab.NumericID(5))
			}
			if len(iids) != 1 || iids[0] != 42 {
				t.Errorf("ProjectMergeRequestsByIIDs called with iids %v, want [42]", iids)
			}
			return []gitlab.MergeRequestSummary{{MergeRequest: fullMR}}, nil
		},
	}
	eventsResult, err := Find(context.Background(), eventsClient, p, Options{Now: func() time.Time { return fixedNow }})
	if err != nil {
		t.Fatalf("Find() (events) error = %v", err)
	}
	if len(eventsResult.Items) != 1 {
		t.Fatalf("Find() (events) returned %d items, want 1 -- items=%+v", len(eventsResult.Items), eventsResult.Items)
	}
	if byIIDsCalls != 1 {
		t.Errorf("ProjectMergeRequestsByIIDs called %d times, want exactly 1", byIIDsCalls)
	}

	if !reflect.DeepEqual(bruteforceResult.Items[0], eventsResult.Items[0]) {
		t.Errorf("strategies disagree on shape for the same merge request:\nbruteforce = %+v\nevents     = %+v", bruteforceResult.Items[0], eventsResult.Items[0])
	}
}

// TestFind_eventsStrategyWithPathProjectScopeResolvesOnce proves the events
// strategy works end to end when --project is given as a namespaced path
// ("my-group/my-project"): scopeProjectSet resolves it via one
// client.GetProject call. Many candidate events, spread across many merge
// requests, prove that resolution happens exactly once per search, never
// once per candidate.
func TestFind_eventsStrategyWithPathProjectScopeResolvesOnce(t *testing.T) {
	const userID = 42
	const numericProjectID = 99
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	projectPath := gitlab.PathID("my-group/my-project")
	p := Params{
		UserID:   userID,
		Range:    mustDateRange(from, to),
		MoreThan: 0,
		Scope:    Scope{ProjectID: &projectPath},
	}
	opts := Options{}

	const wantMergeRequests = 20
	var events []gitlab.CommentEvent
	for iid := int64(1); iid <= wantMergeRequests; iid++ {
		events = append(events, commentEvent(numericProjectID, iid, false, at))
	}
	discussions := []domain.Discussion{discussion("d", note(1, userID, false, at))}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, id int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
		commentEventsFn: func(ctx context.Context, id int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
		getProjectFn: func(ctx context.Context, project gitlab.ID) (domain.Project, error) {
			if project != projectPath {
				t.Errorf("GetProject() called with %s, want %s", project, projectPath)
			}
			return domain.Project{ID: numericProjectID, Path: "my-group/my-project"}, nil
		},
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return discussions, nil
		},
		projectMergeRequestsByIIDsFn: func(ctx context.Context, project gitlab.ID, iids []int64) ([]gitlab.MergeRequestSummary, error) {
			out := make([]gitlab.MergeRequestSummary, len(iids))
			for i, iid := range iids {
				out[i] = gitlab.MergeRequestSummary{MergeRequest: domain.MergeRequest{ProjectID: numericProjectID, IID: iid}}
			}
			return out, nil
		},
	}

	result, err := Find(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if result.Strategy != domain.StrategyEvents {
		t.Fatalf("Find() strategy = %q, want %q", result.Strategy, domain.StrategyEvents)
	}
	if len(result.Items) != wantMergeRequests {
		t.Fatalf("Find() returned %d items, want %d (one per merge request)", len(result.Items), wantMergeRequests)
	}
	if client.getProjectCalls != 1 {
		t.Errorf("GetProject() called %d times, want exactly 1 (resolved once per search, never once per candidate)", client.getProjectCalls)
	}
}

func TestFind_propagatesSelectStrategyError(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}

	wantErr := gitlab.ErrRateLimited
	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, id int64) (domain.SmokeResult, error) {
			return domain.SmokeUnknown, wantErr
		},
	}
	if _, err := Find(context.Background(), client, p, Options{}); err == nil {
		t.Fatal("Find() error = nil, want error")
	}
}
