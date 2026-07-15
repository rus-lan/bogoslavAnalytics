package stats_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/stats"
)

func mustCommentItem(projectID, mrIID int64, at time.Time) artifact.CommentItem {
	return artifact.CommentItem{
		MRIID: mrIID,
		Note: domain.Note{
			ID:        1,
			CreatedAt: at,
			ProjectID: projectID,
		},
	}
}

// TestFromCommentList_matchesHandComputedFixture builds a five-comment
// fixture (2 merge requests across 2 projects, spread over 2 days) and
// asserts the aggregate against counts computed by hand:
//
//	(project 1, mr 10): 2 comments, both on 2026-03-01
//	(project 1, mr 11): 1 comment, on 2026-03-02
//	(project 2, mr 20): 2 comments, one per day
//
// -> by_mr = [{1,10,2}, {1,11,1}, {2,20,2}], by_date = {03-01: 3, 03-02: 2}.
func TestFromCommentList_matchesHandComputedFixture(t *testing.T) {
	day1 := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)

	doc := artifact.CommentList{
		Items: []artifact.CommentItem{
			mustCommentItem(1, 10, day1),
			mustCommentItem(1, 10, day1),
			mustCommentItem(1, 11, day2),
			mustCommentItem(2, 20, day1),
			mustCommentItem(2, 20, day2),
		},
	}

	got := stats.FromCommentList(doc)

	if got.SourceKind != artifact.KindCommentList {
		t.Errorf("SourceKind = %q, want %q", got.SourceKind, artifact.KindCommentList)
	}
	if got.TotalItems != 5 {
		t.Errorf("TotalItems = %d, want 5", got.TotalItems)
	}

	wantByMR := []stats.MRCount{
		{ProjectID: 1, MRIID: 10, Count: 2},
		{ProjectID: 1, MRIID: 11, Count: 1},
		{ProjectID: 2, MRIID: 20, Count: 2},
	}
	if !reflect.DeepEqual(got.ByMR, wantByMR) {
		t.Errorf("ByMR = %+v, want %+v", got.ByMR, wantByMR)
	}

	wantByDate := map[string]int{"2026-03-01": 3, "2026-03-02": 2}
	if !reflect.DeepEqual(got.ByDate, wantByDate) {
		t.Errorf("ByDate = %v, want %v", got.ByDate, wantByDate)
	}

	if len(got.ByLabel) != 0 {
		t.Errorf("ByLabel = %v, want empty (comment_list rows carry no label)", got.ByLabel)
	}
}

func TestFromCommentList_emptyInputIsZeroedNotError(t *testing.T) {
	got := stats.FromCommentList(artifact.CommentList{})
	if got.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", got.TotalItems)
	}
	if len(got.ByMR) != 0 || len(got.ByDate) != 0 || len(got.ByLabel) != 0 {
		t.Errorf("got = %+v, want all breakdowns empty", got)
	}
}
