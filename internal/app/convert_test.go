package app

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
	"github.com/rus-lan/bogoslavAnalytics/internal/search"
)

func TestToSearchParams_pathScopeBecomesPathID(t *testing.T) {
	q := domain.Query{Project: "my-group/repo"}
	got := toSearchParams(q)

	if got.Scope.ProjectID == nil {
		t.Fatalf("Scope.ProjectID = nil, want set")
	}
	want := gitlab.PathID("my-group/repo")
	if *got.Scope.ProjectID != want {
		t.Errorf("Scope.ProjectID = %s, want %s", got.Scope.ProjectID.String(), want.String())
	}
	if got.Scope.GroupID != nil {
		t.Errorf("Scope.GroupID = %v, want nil", got.Scope.GroupID)
	}
}

func TestToSearchParams_numericScopeBecomesNumericID(t *testing.T) {
	q := domain.Query{Group: "123"}
	got := toSearchParams(q)

	if got.Scope.GroupID == nil {
		t.Fatalf("Scope.GroupID = nil, want set")
	}
	want := gitlab.NumericID(123)
	if *got.Scope.GroupID != want {
		t.Errorf("Scope.GroupID = %s, want %s", got.Scope.GroupID.String(), want.String())
	}
	if got.Scope.ProjectID != nil {
		t.Errorf("Scope.ProjectID = %v, want nil", got.Scope.ProjectID)
	}
}

func TestToSearchParams_noScopeStaysUnscoped(t *testing.T) {
	got := toSearchParams(domain.Query{})

	if got.Scope.GroupID != nil || got.Scope.ProjectID != nil {
		t.Errorf("Scope = %+v, want both nil", got.Scope)
	}
}

func TestToSearchParams_projectWinsOverGroup(t *testing.T) {
	q := domain.Query{Group: "my-group", Project: "my-group/repo"}
	got := toSearchParams(q)

	if got.Scope.GroupID != nil {
		t.Errorf("Scope.GroupID = %v, want nil when Project is also set", got.Scope.GroupID)
	}
	if got.Scope.ProjectID == nil {
		t.Fatalf("Scope.ProjectID = nil, want set")
	}
	want := gitlab.PathID("my-group/repo")
	if *got.Scope.ProjectID != want {
		t.Errorf("Scope.ProjectID = %s, want %s", got.Scope.ProjectID.String(), want.String())
	}
}

func TestToSearchParams_carriesUserRangeAndMoreThan(t *testing.T) {
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	q := domain.Query{UserID: 7, From: from, To: to, MoreThan: 9}
	got := toSearchParams(q)

	if got.UserID != 7 {
		t.Errorf("UserID = %d, want 7", got.UserID)
	}
	if got.MoreThan != 9 {
		t.Errorf("MoreThan = %d, want 9", got.MoreThan)
	}
	if got.Range.From != from || got.Range.To != to {
		t.Errorf("Range = %+v, want {%s %s}", got.Range, from, to)
	}
}

func TestToMRList_carriesAllFieldsThrough(t *testing.T) {
	createdAt := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.March, 5, 10, 0, 0, 0, time.UTC)

	header := artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: createdAt}}
	query := domain.Query{GitlabURL: "https://gitlab.example.com", UserID: 42, MoreThan: 5}
	result := search.Result{
		Strategy: domain.StrategyEvents,
		Smoke:    domain.SmokePassed,
		Items: []domain.MergeRequest{
			{
				ProjectID:    123,
				ProjectPath:  "my-group/repo",
				IID:          77,
				Title:        "fix bug",
				WebURL:       "https://gitlab.example.com/my-group/repo/-/merge_requests/77",
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
				CommentCount: 8,
			},
		},
	}

	got := toMRList(header, query, result)

	if len(got.Items) != 1 {
		t.Fatalf("toMRList() Items = %+v, want exactly 1", got.Items)
	}
	want := artifact.MRItem{
		ProjectID:    123,
		ProjectPath:  "my-group/repo",
		MRIID:        77,
		CommentCount: 8,
		Title:        "fix bug",
		WebURL:       "https://gitlab.example.com/my-group/repo/-/merge_requests/77",
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	if got.Items[0] != want {
		t.Errorf("toMRList() Items[0] = %+v, want %+v", got.Items[0], want)
	}
	if got.Query.Strategy != domain.StrategyEvents {
		t.Errorf("toMRList() Query.Strategy = %q, want %q", got.Query.Strategy, domain.StrategyEvents)
	}
	if got.Query.Smoke != domain.SmokePassed {
		t.Errorf("toMRList() Query.Smoke = %q, want %q", got.Query.Smoke, domain.SmokePassed)
	}
	if got.Header != header {
		t.Errorf("toMRList() Header = %+v, want %+v", got.Header, header)
	}
}
