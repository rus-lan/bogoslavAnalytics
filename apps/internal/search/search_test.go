package search

import (
	"context"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
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
		discussionsFn: func(ctx context.Context, projectID, mrIID int64) ([]domain.Discussion, error) {
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

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, id int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
		commentEventsFn: func(ctx context.Context, id int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
			return events, nil
		},
		discussionsFn: func(ctx context.Context, projectID, mrIID int64) ([]domain.Discussion, error) {
			return discussions, nil
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
	if result.Items[0].CommentCount != 2 {
		t.Errorf("Find() comment_count = %d, want 2", result.Items[0].CommentCount)
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
