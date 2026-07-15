package stats_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/stats"
)

func mustLabeledItem(projectID, mrIID int64, at time.Time, label string) artifact.LabeledCommentItem {
	return artifact.LabeledCommentItem{
		MRIID: mrIID,
		LabeledNote: domain.LabeledNote{
			Note: domain.Note{
				ID:        1,
				CreatedAt: at,
				ProjectID: projectID,
			},
			Label: label,
		},
	}
}

// labeledFixture builds a six-comment fixture (2 projects, 3 merge
// requests, 2 days, 3 labels) whose aggregate is computed by hand:
//
//	(project 1, mr 10): 2 rows, both "bug", both on day 1
//	(project 1, mr 11): 1 row, "style", on day 2
//	(project 2, mr 20): 3 rows: 1 "bug" on day 1, 2 "praise" on day 2
//
// -> by_mr = [{1,10,2}, {1,11,1}, {2,20,3}]
// -> by_label = {bug: 3, style: 1, praise: 2}
// -> by_date = {day1: 3, day2: 3}
func labeledFixture() []artifact.LabeledCommentItem {
	day1 := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)
	return []artifact.LabeledCommentItem{
		mustLabeledItem(1, 10, day1, "bug"),
		mustLabeledItem(1, 10, day1, "bug"),
		mustLabeledItem(1, 11, day2, "style"),
		mustLabeledItem(2, 20, day1, "bug"),
		mustLabeledItem(2, 20, day2, "praise"),
		mustLabeledItem(2, 20, day2, "praise"),
	}
}

func TestFromLabeledComments_matchesHandComputedFixture(t *testing.T) {
	doc := artifact.LabeledComments{Items: labeledFixture()}
	got := stats.FromLabeledComments(doc)

	if got.SourceKind != artifact.KindLabeledComments {
		t.Errorf("SourceKind = %q, want %q", got.SourceKind, artifact.KindLabeledComments)
	}
	if got.TotalItems != 6 {
		t.Errorf("TotalItems = %d, want 6", got.TotalItems)
	}

	wantByMR := []stats.MRCount{
		{ProjectID: 1, MRIID: 10, Count: 2},
		{ProjectID: 1, MRIID: 11, Count: 1},
		{ProjectID: 2, MRIID: 20, Count: 3},
	}
	if !reflect.DeepEqual(got.ByMR, wantByMR) {
		t.Errorf("ByMR = %+v, want %+v", got.ByMR, wantByMR)
	}

	wantByLabel := map[string]int{"bug": 3, "style": 1, "praise": 2}
	if !reflect.DeepEqual(got.ByLabel, wantByLabel) {
		t.Errorf("ByLabel = %v, want %v", got.ByLabel, wantByLabel)
	}

	wantByDate := map[string]int{"2026-03-01": 3, "2026-03-02": 3}
	if !reflect.DeepEqual(got.ByDate, wantByDate) {
		t.Errorf("ByDate = %v, want %v", got.ByDate, wantByDate)
	}
}

func TestFromLabeledComments_emptyInputIsZeroedNotError(t *testing.T) {
	got := stats.FromLabeledComments(artifact.LabeledComments{})
	if got.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", got.TotalItems)
	}
	if len(got.ByMR) != 0 || len(got.ByLabel) != 0 || len(got.ByDate) != 0 {
		t.Errorf("got = %+v, want all breakdowns empty", got)
	}
}
