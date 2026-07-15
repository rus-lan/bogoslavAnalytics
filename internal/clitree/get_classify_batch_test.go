package clitree

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/internal/classify"
)

func TestNewGetClassifyBatchRequest_mapsFlagsToRequest(t *testing.T) {
	cmd, flags := buildFlags(t, registerGetClassifyBatchFlags, []string{
		"--from-artifact", "artifacts/comment_list_x.yaml",
		"--model", "glm-5.2",
		"--artifacts-dir", "out",
	})
	_ = cmd

	got, err := newGetClassifyBatchRequest(*flags)
	if err != nil {
		t.Fatalf("newGetClassifyBatchRequest() error = %v", err)
	}
	want := app.GetClassifyBatchRequest{
		CommentListPath: "artifacts/comment_list_x.yaml",
		Model:           "glm-5.2",
		Dir:             "out",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newGetClassifyBatchRequest() = %+v, want %+v", got, want)
	}
}

func TestNewGetClassifyBatchRequest_loadsCustomTaxonomyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "taxonomy.json")
	writeFileT(t, path, `{"version": 2, "labels": ["bug", "other"]}`)

	_, flags := buildFlags(t, registerGetClassifyBatchFlags, []string{
		"--from-artifact", "x.yaml",
		"--model", "glm-5.2",
		"--taxonomy-file", path,
	})

	got, err := newGetClassifyBatchRequest(*flags)
	if err != nil {
		t.Fatalf("newGetClassifyBatchRequest() error = %v", err)
	}
	if got.Taxonomy == nil {
		t.Fatal("Taxonomy = nil, want the loaded custom taxonomy")
	}
	want := classify.Taxonomy{Version: 2, Labels: []string{"bug", "other"}}
	if !reflect.DeepEqual(*got.Taxonomy, want) {
		t.Errorf("Taxonomy = %+v, want %+v", *got.Taxonomy, want)
	}
}

func TestNewGetClassifyBatchRequest_rejectsTaxonomyMissingOther(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "taxonomy.json")
	writeFileT(t, path, `{"version": 2, "labels": ["bug"]}`)

	_, flags := buildFlags(t, registerGetClassifyBatchFlags, []string{
		"--from-artifact", "x.yaml",
		"--model", "glm-5.2",
		"--taxonomy-file", path,
	})

	_, err := newGetClassifyBatchRequest(*flags)
	if err == nil {
		t.Fatal("newGetClassifyBatchRequest() error = nil, want an error: taxonomy is missing the required \"other\" label")
	}
}
