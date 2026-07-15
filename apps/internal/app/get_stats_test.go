package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/stats"
)

func TestGetStats_aggregatesMRList(t *testing.T) {
	dir := t.TempDir()
	doc := artifact.MRList{
		Header: artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: time.Now()}},
		Query: domain.Query{
			UserID: 42,
			From:   domain.NewDate(2026, time.January, 1),
			To:     domain.NewDate(2026, time.June, 30),
		},
		Items: []artifact.MRItem{{ProjectID: 1, MRIID: 7, CommentCount: 3}},
	}
	path := filepath.Join(dir, "mr_list_test.yaml")
	if err := artifact.WriteMRList(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	result, err := GetStats(GetStatsRequest{ArtifactPath: path})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	want := stats.FromMRList(doc)
	if result.Stats.TotalItems != want.TotalItems || result.Stats.SourceKind != want.SourceKind {
		t.Errorf("GetStats() = %+v, want %+v", result.Stats, want)
	}
	if result.Path != "" {
		t.Errorf("Path = %q, want empty (Dir not set)", result.Path)
	}
}

func TestGetStats_aggregatesLabeledCommentsAndWritesWhenDirSet(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	doc := artifact.LabeledComments{
		Header: artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: at}},
		Query: artifact.CommentQuery{
			UserID: 42,
			From:   domain.NewDate(2026, time.January, 1),
			To:     domain.NewDate(2026, time.June, 30),
		},
		Taxonomy:   artifact.Taxonomy{Version: 1, Labels: []string{"bug", "other"}},
		Classifier: domain.Classifier{Tool: "opencode", Model: "m", TaxonomyVersion: 1, ClassifiedAt: at},
		Items: []artifact.LabeledCommentItem{
			{MRIID: 7, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}, Label: "bug"}},
		},
	}
	path := filepath.Join(dir, "labeled_comments_test.yaml")
	if err := artifact.WriteLabeledComments(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	outDir := filepath.Join(dir, "out")
	result, err := GetStats(GetStatsRequest{ArtifactPath: path, Dir: outDir, Format: artifact.FormatJSON})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if result.Stats.SourceKind != artifact.KindLabeledComments {
		t.Errorf("SourceKind = %q, want labeled_comments", result.Stats.SourceKind)
	}
	if result.Stats.ByLabel["bug"] != 1 {
		t.Errorf("ByLabel[bug] = %d, want 1", result.Stats.ByLabel["bug"])
	}
	if result.Path == "" {
		t.Fatalf("Path = empty, want set since Dir was set")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Errorf("os.Stat(%q) error = %v", result.Path, err)
	}
}

func TestGetStats_detectsKindForAllFourArtifactKinds(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)

	mrListPath := filepath.Join(dir, "a_mr_list.yaml")
	mrListDoc := artifact.MRList{
		Query: domain.Query{From: from, To: to},
		Items: []artifact.MRItem{{ProjectID: 1, MRIID: 1}},
	}
	if err := artifact.WriteMRList(mrListDoc, artifact.FormatYAML, mrListPath); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	commentListPath := filepath.Join(dir, "b_comment_list.yaml")
	commentListDoc := artifact.CommentList{
		Query: artifact.CommentQuery{From: from, To: to},
		Items: []artifact.CommentItem{{MRIID: 1, Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}}},
	}
	if err := artifact.WriteCommentList(commentListDoc, artifact.FormatYAML, commentListPath); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	labeledPath := filepath.Join(dir, "c_labeled_comments.yaml")
	labeledDoc := artifact.LabeledComments{
		Query:      artifact.CommentQuery{From: from, To: to},
		Classifier: domain.Classifier{Tool: "t", Model: "m", TaxonomyVersion: 1, ClassifiedAt: at},
		Items:      []artifact.LabeledCommentItem{{MRIID: 1, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}, Label: "bug"}}},
	}
	if err := artifact.WriteLabeledComments(labeledDoc, artifact.FormatYAML, labeledPath); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	filteredPath := filepath.Join(dir, "d_filtered_comments.yaml")
	filteredDoc := artifact.FilteredComments{
		Query: artifact.FilteredQuery{FromArtifact: labeledPath, Labels: []string{"bug"}},
		Items: []artifact.LabeledCommentItem{{MRIID: 1, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}, Label: "bug"}}},
	}
	if err := artifact.WriteFilteredComments(filteredDoc, artifact.FormatYAML, filteredPath); err != nil {
		t.Fatalf("WriteFilteredComments() error = %v", err)
	}

	cases := []struct {
		path string
		want artifact.Kind
	}{
		{mrListPath, artifact.KindMRList},
		{commentListPath, artifact.KindCommentList},
		{labeledPath, artifact.KindLabeledComments},
		{filteredPath, artifact.KindFilteredComments},
	}
	for _, tc := range cases {
		result, err := GetStats(GetStatsRequest{ArtifactPath: tc.path})
		if err != nil {
			t.Fatalf("GetStats(%q) error = %v", tc.path, err)
		}
		if result.Stats.SourceKind != tc.want {
			t.Errorf("GetStats(%q) SourceKind = %q, want %q", tc.path, result.Stats.SourceKind, tc.want)
		}
	}
}
