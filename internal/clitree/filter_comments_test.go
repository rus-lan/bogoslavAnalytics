package clitree

import (
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func TestNewFilterCommentsRequest_mapsFlagsToRequest(t *testing.T) {
	_, flags := buildFlags(t, registerFilterCommentsFlags, []string{
		"--from-artifact", "artifacts/labeled_comments_x.yaml",
		"--label", "bug",
		"--label", "style",
		"--group", "my-group",
		"--format", "json",
	})

	got, err := newFilterCommentsRequest(*flags, nil, nil, []int64{1, 2, 3}, nil)
	if err != nil {
		t.Fatalf("newFilterCommentsRequest() error = %v", err)
	}
	want := app.FilterCommentsRequest{
		LabeledCommentsPath: "artifacts/labeled_comments_x.yaml",
		Labels:              []string{"bug", "style"},
		Group:               "my-group",
		ProjectIDs:          []int64{1, 2, 3},
		Format:              artifact.FormatJSON,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newFilterCommentsRequest() = %+v, want %+v", got, want)
	}
}

func TestParseOptionalDateRange(t *testing.T) {
	t.Run("both empty returns nil, nil", func(t *testing.T) {
		from, to, err := parseOptionalDateRange("", "")
		if err != nil {
			t.Fatalf("parseOptionalDateRange() error = %v", err)
		}
		if from != nil || to != nil {
			t.Errorf("parseOptionalDateRange() = (%v, %v), want (nil, nil)", from, to)
		}
	})

	t.Run("both set returns parsed pointers", func(t *testing.T) {
		from, to, err := parseOptionalDateRange("2026-01-01", "2026-06-30")
		if err != nil {
			t.Fatalf("parseOptionalDateRange() error = %v", err)
		}
		wantFrom := domain.NewDate(2026, time.January, 1)
		wantTo := domain.NewDate(2026, time.June, 30)
		if from == nil || *from != wantFrom {
			t.Errorf("from = %v, want %v", from, wantFrom)
		}
		if to == nil || *to != wantTo {
			t.Errorf("to = %v, want %v", to, wantTo)
		}
	})

	t.Run("unparsable from is rejected", func(t *testing.T) {
		_, _, err := parseOptionalDateRange("not-a-date", "2026-06-30")
		if err == nil {
			t.Fatal("parseOptionalDateRange() error = nil, want an error")
		}
	})
}

func TestResolveProjectID_numericSkipsGitlabCall(t *testing.T) {
	// A nil *gitlab.Client would panic if resolveProjectID ever tried to
	// call a method on it; passing nil here proves the all-digits path
	// never touches the client at all.
	got, err := resolveProjectID(nil, nil, "123")
	if err != nil {
		t.Fatalf("resolveProjectID() error = %v", err)
	}
	if got != 123 {
		t.Errorf("resolveProjectID() = %d, want 123", got)
	}
}

func TestNewFilterCommentsRequest_rejectsUnknownFormat(t *testing.T) {
	_, flags := buildFlags(t, registerFilterCommentsFlags, []string{
		"--from-artifact", "x.yaml", "--label", "bug", "--format", "xml",
	})
	_, err := newFilterCommentsRequest(*flags, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("newFilterCommentsRequest() error = nil, want an error for --format xml")
	}
}
