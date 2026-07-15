package filter_test

import (
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/filter"
)

func mrItem(projectID, mrIID int64, commentCount int) artifact.MRItem {
	return artifact.MRItem{
		ProjectID:    projectID,
		ProjectPath:  "my-group/repo",
		MRIID:        mrIID,
		CommentCount: commentCount,
	}
}

func TestMRsByCount_thresholdIsStrictlyGreater(t *testing.T) {
	cases := []struct {
		name         string
		commentCount int
		moreThan     int
		want         bool
	}{
		{"exactly N is excluded", 5, 5, false},
		{"N+1 is included", 6, 5, true},
		{"below N is excluded", 4, 5, false},
		{"zero threshold includes any comment", 1, 0, true},
		{"zero threshold excludes zero comments", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []artifact.MRItem{mrItem(1, 1, tc.commentCount)}
			got := filter.MRsByCount(items, tc.moreThan)
			if (len(got) == 1) != tc.want {
				t.Errorf("MRsByCount(count=%d, moreThan=%d) len = %d, want included=%v", tc.commentCount, tc.moreThan, len(got), tc.want)
			}
		})
	}
}

func TestMRsByCount_emptyInputReturnsEmpty(t *testing.T) {
	got := filter.MRsByCount(nil, 5)
	if len(got) != 0 {
		t.Errorf("MRsByCount(nil) = %v, want empty", got)
	}
}

func TestMRsByProject_keepsOnlyMatchingProject(t *testing.T) {
	items := []artifact.MRItem{
		mrItem(1, 10, 8),
		mrItem(2, 20, 8),
		mrItem(1, 11, 8),
	}
	got := filter.MRsByProject(items, 1)
	if len(got) != 2 {
		t.Fatalf("MRsByProject() len = %d, want 2", len(got))
	}
	for _, it := range got {
		if it.ProjectID != 1 {
			t.Errorf("MRsByProject() kept ProjectID = %d, want 1", it.ProjectID)
		}
	}
}

func TestMRsByGroup_keepsOnlyProjectsInSet(t *testing.T) {
	items := []artifact.MRItem{
		mrItem(1, 10, 8),
		mrItem(2, 20, 8),
		mrItem(3, 30, 8),
	}
	got := filter.MRsByGroup(items, []int64{1, 3})
	if len(got) != 2 {
		t.Fatalf("MRsByGroup() len = %d, want 2", len(got))
	}
	for _, it := range got {
		if it.ProjectID != 1 && it.ProjectID != 3 {
			t.Errorf("MRsByGroup() kept ProjectID = %d, want 1 or 3", it.ProjectID)
		}
	}
}

func TestMRsByGroup_emptySetYieldsEmpty(t *testing.T) {
	items := []artifact.MRItem{mrItem(1, 10, 8)}
	got := filter.MRsByGroup(items, nil)
	if len(got) != 0 {
		t.Errorf("MRsByGroup(nil set) = %v, want empty", got)
	}
}

func TestMRPoint_returnsSingleMatch(t *testing.T) {
	items := []artifact.MRItem{
		mrItem(1, 10, 8),
		mrItem(1, 11, 8),
		mrItem(2, 10, 8),
	}
	got := filter.MRPoint(items, 1, 11)
	if len(got) != 1 {
		t.Fatalf("MRPoint() len = %d, want 1", len(got))
	}
	if got[0].ProjectID != 1 || got[0].MRIID != 11 {
		t.Errorf("MRPoint() = %+v, want project 1 mr 11", got[0])
	}
}

func TestMRPoint_noMatchReturnsEmpty(t *testing.T) {
	items := []artifact.MRItem{mrItem(1, 10, 8)}
	got := filter.MRPoint(items, 1, 99)
	if len(got) != 0 {
		t.Errorf("MRPoint() = %v, want empty", got)
	}
}

func TestMRsFilters_compose(t *testing.T) {
	items := []artifact.MRItem{
		mrItem(1, 10, 8),  // project 1, count 8 > 5, kept
		mrItem(1, 11, 5),  // project 1, count 5 == 5, dropped by count
		mrItem(2, 20, 10), // project 2, dropped by project
	}
	got := filter.MRsByProject(filter.MRsByCount(items, 5), 1)
	if len(got) != 1 {
		t.Fatalf("composed filters len = %d, want 1", len(got))
	}
	if got[0].MRIID != 10 {
		t.Errorf("composed filters kept mr_iid %d, want 10", got[0].MRIID)
	}
}
