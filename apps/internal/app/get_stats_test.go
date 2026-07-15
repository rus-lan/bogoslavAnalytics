package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/stats"
)

// statsFixture is one input artifact that exercises every field of
// stats.Stats: SourceKind, TotalItems, ByMR (with both ProjectID and
// MRIID), ByLabel, and ByDate. labeled_comments is the only one of the
// four artifact kinds that fills all of those.
func statsFixture(t *testing.T, dir string) string {
	t.Helper()
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
			{MRIID: 7, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 3, CreatedAt: at}, Label: "bug"}},
		},
	}
	path := filepath.Join(dir, "labeled_comments_fixture.yaml")
	if err := artifact.WriteLabeledComments(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}
	return path
}

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

// TestGetStats_yamlFileUsesUnderscoredFieldNames guards against writeStats
// calling yaml.Marshal(s) directly: stats.Stats carries only json struct
// tags, so gopkg.in/yaml.v3 would ignore them and lowercase the bare Go
// field names instead (source_kind -> sourcekind, by_mr -> bymr, and so
// on), disagreeing with every other rendering of the same value.
func TestGetStats_yamlFileUsesUnderscoredFieldNames(t *testing.T) {
	dir := t.TempDir()
	path := statsFixture(t, dir)

	outDir := filepath.Join(dir, "out")
	result, err := GetStats(GetStatsRequest{ArtifactPath: path, Dir: outDir, Format: artifact.FormatYAML})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	raw, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", result.Path, err)
	}
	text := string(raw)

	for _, want := range []string{"source_kind", "total_items", "by_mr", "project_id", "mr_iid", "by_label", "by_date"} {
		if !strings.Contains(text, want) {
			t.Errorf("written yaml file does not contain %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{"sourcekind", "totalitems", "bymr", "bylabel", "bydate", "projectid", "mriid"} {
		if strings.Contains(text, notWant) {
			t.Errorf("written yaml file must not contain %q (a field name with a missing underscore):\n%s", notWant, text)
		}
	}
}

// collectFieldNames recursively walks a value decoded from JSON or YAML
// into `any` and gathers every map key it finds, so two decoded values
// can be compared on field names alone, independent of the number types
// encoding/json (float64) and gopkg.in/yaml.v3 (int/uint64) each produce
// for the same document.
func collectFieldNames(v any, keys map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			keys[k] = true
			collectFieldNames(val, keys)
		}
	case []any:
		for _, item := range t {
			collectFieldNames(item, keys)
		}
	}
}

// TestGetStats_yamlAndJSONFilesDescribeSameFieldNames asserts the
// invariant that was broken: writeStats must describe the same aggregate
// with the same field names regardless of --format, matching how
// get-stats renders the very same stats.Stats value to stdout
// (internal/clitree's marshalJSONOrYAML) by going through JSON first in
// both cases.
func TestGetStats_yamlAndJSONFilesDescribeSameFieldNames(t *testing.T) {
	dir := t.TempDir()
	path := statsFixture(t, dir)

	yamlResult, err := GetStats(GetStatsRequest{ArtifactPath: path, Dir: filepath.Join(dir, "yaml-out"), Format: artifact.FormatYAML})
	if err != nil {
		t.Fatalf("GetStats(yaml) error = %v", err)
	}
	jsonResult, err := GetStats(GetStatsRequest{ArtifactPath: path, Dir: filepath.Join(dir, "json-out"), Format: artifact.FormatJSON})
	if err != nil {
		t.Fatalf("GetStats(json) error = %v", err)
	}

	yamlRaw, err := os.ReadFile(yamlResult.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", yamlResult.Path, err)
	}
	jsonRaw, err := os.ReadFile(jsonResult.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", jsonResult.Path, err)
	}

	var fromYAML, fromJSON any
	if err := yaml.Unmarshal(yamlRaw, &fromYAML); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if err := json.Unmarshal(jsonRaw, &fromJSON); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	yamlKeys, jsonKeys := map[string]bool{}, map[string]bool{}
	collectFieldNames(fromYAML, yamlKeys)
	collectFieldNames(fromJSON, jsonKeys)
	if !reflect.DeepEqual(yamlKeys, jsonKeys) {
		t.Errorf("yaml file field names = %v, json file field names = %v", yamlKeys, jsonKeys)
	}
}

// TestGetStats_jsonFileRoundTrips is the no-regression check: the json
// write path was already correct, and must stay that way.
func TestGetStats_jsonFileRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := statsFixture(t, dir)

	outDir := filepath.Join(dir, "out")
	result, err := GetStats(GetStatsRequest{ArtifactPath: path, Dir: outDir, Format: artifact.FormatJSON})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	raw, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", result.Path, err)
	}

	var got stats.Stats
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, result.Stats) {
		t.Errorf("round-tripped json stats = %+v, want %+v", got, result.Stats)
	}
}
