package filter_test

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/filter"
)

func labeledItem(projectID, mrIID int64, createdAt time.Time, label string) artifact.LabeledCommentItem {
	return artifact.LabeledCommentItem{
		MRIID: mrIID,
		LabeledNote: domain.LabeledNote{
			Note: domain.Note{
				ID:        1,
				Body:      "text",
				CreatedAt: createdAt,
				ProjectID: projectID,
			},
			Label: label,
		},
	}
}

func TestByDate_boundaryInstants(t *testing.T) {
	r, err := domain.NewDateRange(
		domain.NewDate(2026, time.January, 1),
		domain.NewDate(2026, time.June, 30),
	)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.June, 30, 23, 59, 59, 999_000_000, time.UTC)

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"exactly start instant is included", start, true},
		{"exactly end instant is included", end, true},
		{"one millisecond before start is excluded", start.Add(-time.Millisecond), false},
		{"one millisecond after end is excluded", end.Add(time.Millisecond), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []artifact.LabeledCommentItem{labeledItem(1, 10, tc.at, "bug")}
			got := filter.ByDate(items, r)
			if (len(got) == 1) != tc.want {
				t.Errorf("ByDate(%v) len = %d, want included=%v", tc.at, len(got), tc.want)
			}
		})
	}
}

func TestByProject_keepsOnlyMatchingProject(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{
		labeledItem(1, 10, now, "bug"),
		labeledItem(2, 20, now, "bug"),
	}
	got := filter.ByProject(items, 1)
	if len(got) != 1 {
		t.Fatalf("ByProject() len = %d, want 1", len(got))
	}
	if got[0].ProjectID != 1 {
		t.Errorf("ByProject() kept ProjectID = %d, want 1", got[0].ProjectID)
	}
}

func TestByGroup_keepsOnlyProjectsInSet(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{
		labeledItem(1, 10, now, "bug"),
		labeledItem(2, 20, now, "bug"),
		labeledItem(3, 30, now, "bug"),
	}
	got := filter.ByGroup(items, []int64{1, 3})
	if len(got) != 2 {
		t.Fatalf("ByGroup() len = %d, want 2", len(got))
	}
}

func TestByLabel_nonexistentLabelYieldsEmptyNotError(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{
		labeledItem(1, 10, now, "bug"),
		labeledItem(1, 11, now, "style"),
	}
	got := filter.ByLabel(items, "does-not-exist")
	if got == nil {
		t.Fatal("ByLabel() = nil, want a non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ByLabel() = %v, want empty", got)
	}
}

func TestByLabel_matchesLabelGroup(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{
		labeledItem(1, 10, now, "bug"),
		labeledItem(1, 11, now, "style"),
		labeledItem(1, 12, now, "praise"),
	}
	got := filter.ByLabel(items, "bug", "style")
	if len(got) != 2 {
		t.Fatalf("ByLabel(bug, style) len = %d, want 2", len(got))
	}
	for _, it := range got {
		if it.Label != "bug" && it.Label != "style" {
			t.Errorf("ByLabel() kept label %q, want bug or style", it.Label)
		}
	}
}

func TestByLabel_noLabelsGivenYieldsEmpty(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{labeledItem(1, 10, now, "bug")}
	got := filter.ByLabel(items)
	if len(got) != 0 {
		t.Errorf("ByLabel() with no labels = %v, want empty", got)
	}
}

func TestLabeledFilters_compose(t *testing.T) {
	inRange := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	items := []artifact.LabeledCommentItem{
		labeledItem(1, 10, inRange, "bug"),    // kept: project 1, in range, label bug
		labeledItem(1, 11, outOfRange, "bug"), // dropped: out of range
		labeledItem(2, 20, inRange, "bug"),    // dropped: wrong project
		labeledItem(1, 12, inRange, "praise"), // dropped: wrong label
	}
	r, err := domain.NewDateRange(
		domain.NewDate(2026, time.January, 1),
		domain.NewDate(2026, time.June, 30),
	)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	got := filter.ByLabel(filter.ByProject(filter.ByDate(items, r), 1), "bug")
	if len(got) != 1 {
		t.Fatalf("composed filters len = %d, want 1", len(got))
	}
	if got[0].MRIID != 10 {
		t.Errorf("composed filters kept mr_iid %d, want 10", got[0].MRIID)
	}
}
