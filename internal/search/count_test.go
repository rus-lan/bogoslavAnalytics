package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

func TestCountComments_usesDiscussionsDataAndMatchesHandComputedFixture(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	r := mustDateRange(from, to)

	inside := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	outsideBefore := time.Date(2026, time.February, 20, 12, 0, 0, 0, time.UTC)

	discussions := []domain.Discussion{
		// Thread with two replies from the target user (both inside the
		// range) and one from another user: the hand-computed count for
		// this thread is 2.
		discussion("d1",
			note(1, 42, false, inside),
			note(2, 42, false, inside.Add(time.Hour)),
			note(3, 43, false, inside),
		),
		// A system note from the target user must never be counted.
		discussion("d2", note(4, 42, true, inside)),
		// A note from the target user outside the range must never be
		// counted.
		discussion("d3", note(5, 42, false, outsideBefore)),
		// A single-note thread from the target user inside the range: +1.
		discussion("d4", note(6, 42, false, inside)),
	}
	// Hand-computed exact count: notes 1, 2, 6 -> 3.
	const want = 3

	var gotProject gitlab.ID
	var gotMRIID int64
	client := &fakeClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			gotProject, gotMRIID = project, mrIID
			return discussions, nil
		},
	}

	got, err := CountComments(context.Background(), client, 123, 77, 42, r)
	if err != nil {
		t.Fatalf("CountComments() error = %v", err)
	}
	if got != want {
		t.Errorf("CountComments() = %d, want %d", got, want)
	}
	if gotProject != gitlab.NumericID(123) || gotMRIID != 77 {
		t.Errorf("Discussions() called with project=%s mr=%d, want project=123 mr=77", gotProject, gotMRIID)
	}
}

func TestCountComments_neverCallsNotesEndpoint(t *testing.T) {
	// fakeClient has no notes-shaped method at all: Client only exposes
	// Discussions. This test documents that guarantee at the type level --
	// if a /notes-based method were ever added to the Client interface,
	// this test would need updating to prove CountComments still avoids
	// it.
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.January, 31)
	r := mustDateRange(from, to)

	called := false
	client := &fakeClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			called = true
			return nil, nil
		},
	}
	if _, err := CountComments(context.Background(), client, 1, 1, 1, r); err != nil {
		t.Fatalf("CountComments() error = %v", err)
	}
	if !called {
		t.Error("CountComments() did not call Discussions")
	}
}

func TestCountComments_propagatesDiscussionsError(t *testing.T) {
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.January, 31)
	r := mustDateRange(from, to)

	wantErr := gitlab.ErrRateLimited
	client := &fakeClient{
		discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
			return nil, wantErr
		},
	}
	if _, err := CountComments(context.Background(), client, 1, 1, 1, r); !errors.Is(err, wantErr) {
		t.Fatalf("CountComments() error = %v, want wrapping %v", err, wantErr)
	}
}
