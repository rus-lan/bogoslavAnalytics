package filter_test

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/filter"
)

func commentItem(mrIID int64, createdAt time.Time) artifact.CommentItem {
	return artifact.CommentItem{
		MRIID: mrIID,
		Note: domain.Note{
			ID:        1,
			Body:      "text",
			CreatedAt: createdAt,
			ProjectID: 123,
		},
	}
}

func TestCommentsByDate_boundaryInstants(t *testing.T) {
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
		{"mid range is included", time.Date(2026, time.March, 15, 12, 0, 0, 0, time.UTC), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []artifact.CommentItem{commentItem(1, tc.at)}
			got := filter.CommentsByDate(items, r)
			if (len(got) == 1) != tc.want {
				t.Errorf("CommentsByDate(%v) len = %d, want included=%v", tc.at, len(got), tc.want)
			}
		})
	}
}

func TestCommentsByDate_emptyInputReturnsEmpty(t *testing.T) {
	r, _ := domain.NewDateRange(domain.NewDate(2026, time.January, 1), domain.NewDate(2026, time.June, 30))
	got := filter.CommentsByDate(nil, r)
	if len(got) != 0 {
		t.Errorf("CommentsByDate(nil) = %v, want empty", got)
	}
}
