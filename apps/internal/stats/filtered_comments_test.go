package stats_test

import (
	"reflect"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/stats"
)

// TestFromFilteredComments_matchesHandComputedFixture reuses the same
// hand-computed fixture as TestFromLabeledComments_matchesHandComputedFixture:
// filtered_comments rows share the LabeledCommentItem shape, so the
// aggregate must match exactly.
func TestFromFilteredComments_matchesHandComputedFixture(t *testing.T) {
	doc := artifact.FilteredComments{Items: labeledFixture()}
	got := stats.FromFilteredComments(doc)

	if got.SourceKind != artifact.KindFilteredComments {
		t.Errorf("SourceKind = %q, want %q", got.SourceKind, artifact.KindFilteredComments)
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

func TestFromFilteredComments_emptyInputIsZeroedNotError(t *testing.T) {
	got := stats.FromFilteredComments(artifact.FilteredComments{})
	if got.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", got.TotalItems)
	}
	if len(got.ByMR) != 0 || len(got.ByLabel) != 0 || len(got.ByDate) != 0 {
		t.Errorf("got = %+v, want all breakdowns empty", got)
	}
}
