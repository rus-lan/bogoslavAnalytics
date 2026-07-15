package stats_test

import (
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/stats"
)

func TestFromMRList_countsItemsOnly(t *testing.T) {
	doc := artifact.MRList{
		Items: []artifact.MRItem{
			{ProjectID: 1, ProjectPath: "g/repo", MRIID: 10, CommentCount: 8},
			{ProjectID: 1, ProjectPath: "g/repo", MRIID: 11, CommentCount: 3},
			{ProjectID: 2, ProjectPath: "g/other", MRIID: 20, CommentCount: 6},
		},
	}
	got := stats.FromMRList(doc)

	if got.SourceKind != artifact.KindMRList {
		t.Errorf("SourceKind = %q, want %q", got.SourceKind, artifact.KindMRList)
	}
	if got.TotalItems != 3 {
		t.Errorf("TotalItems = %d, want 3", got.TotalItems)
	}
	if len(got.ByMR) != 0 {
		t.Errorf("ByMR = %v, want empty", got.ByMR)
	}
	if len(got.ByLabel) != 0 {
		t.Errorf("ByLabel = %v, want empty", got.ByLabel)
	}
	if len(got.ByDate) != 0 {
		t.Errorf("ByDate = %v, want empty", got.ByDate)
	}
}

func TestFromMRList_emptyInputIsZeroedNotError(t *testing.T) {
	got := stats.FromMRList(artifact.MRList{})
	if got.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", got.TotalItems)
	}
	if len(got.ByMR) != 0 || len(got.ByLabel) != 0 || len(got.ByDate) != 0 {
		t.Errorf("got = %+v, want all breakdowns empty", got)
	}
}
