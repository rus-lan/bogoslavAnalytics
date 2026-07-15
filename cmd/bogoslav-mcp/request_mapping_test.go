package main

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/classify"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/mcptool"
)

// TestNewFindMRsRequest_mapsEveryField is the acceptance check for
// TZ.md section 7.3 on the find_mrs tool: every FindMRsInput field lands
// on the matching app.FindMRsRequest field, MR is only set when non-zero
// (point mode, TZ.md sections 1.2, 7.2), and cache_ttl_seconds/refresh
// become app.CacheOptions.
func TestNewFindMRsRequest_mapsEveryField(t *testing.T) {
	in := mcptool.FindMRsInput{
		User:            "alice",
		From:            "2026-01-01",
		To:              "2026-06-30",
		MoreThan:        5,
		Group:           "my-group",
		Project:         "my-group/repo",
		MR:              77,
		Strict:          true,
		ArtifactsDir:    "out",
		Format:          "json",
		Refresh:         true,
		CacheTTLSeconds: 3600,
	}

	req, err := newFindMRsRequest(in, "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("newFindMRsRequest() error = %v", err)
	}

	wantFrom, _ := domain.ParseDate("2026-01-01")
	wantTo, _ := domain.ParseDate("2026-06-30")

	if req.GitlabURL != "https://gitlab.example.com" {
		t.Errorf("GitlabURL = %q, want %q", req.GitlabURL, "https://gitlab.example.com")
	}
	if req.User != "alice" {
		t.Errorf("User = %q, want %q", req.User, "alice")
	}
	if req.From != wantFrom || req.To != wantTo {
		t.Errorf("From/To = %v/%v, want %v/%v", req.From, req.To, wantFrom, wantTo)
	}
	if req.MoreThan != 5 {
		t.Errorf("MoreThan = %d, want 5", req.MoreThan)
	}
	if req.Group != "my-group" || req.Project != "my-group/repo" {
		t.Errorf("Group/Project = %q/%q, want %q/%q", req.Group, req.Project, "my-group", "my-group/repo")
	}
	if req.MR == nil || *req.MR != 77 {
		t.Errorf("MR = %v, want pointer to 77", req.MR)
	}
	if !req.Strict {
		t.Error("Strict = false, want true")
	}
	if req.Dir != "out" {
		t.Errorf("Dir = %q, want %q", req.Dir, "out")
	}
	if req.Format != artifact.FormatJSON {
		t.Errorf("Format = %q, want %q", req.Format, artifact.FormatJSON)
	}
	if !req.Cache.Refresh {
		t.Error("Cache.Refresh = false, want true")
	}
	if req.Cache.TTL != time.Hour {
		t.Errorf("Cache.TTL = %v, want 1h", req.Cache.TTL)
	}
}

// TestNewFindMRsRequest_zeroMRMeansNoPointMode confirms mr: 0 (the JSON
// zero value for an omitted field) does not set app.FindMRsRequest.MR:
// GitLab merge request iids start at 1, so 0 unambiguously means "not
// set" (TZ.md sections 1.2, 7.2).
func TestNewFindMRsRequest_zeroMRMeansNoPointMode(t *testing.T) {
	in := mcptool.FindMRsInput{User: "1", From: "2026-01-01", To: "2026-01-02"}

	req, err := newFindMRsRequest(in, "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("newFindMRsRequest() error = %v", err)
	}
	if req.MR != nil {
		t.Errorf("MR = %v, want nil", req.MR)
	}
}

// TestNewFindMRsRequest_rejectsBadFormat confirms an unknown format
// value is rejected before an app.FindMRsRequest is even built.
func TestNewFindMRsRequest_rejectsBadFormat(t *testing.T) {
	in := mcptool.FindMRsInput{User: "1", From: "2026-01-01", To: "2026-01-02", Format: "csv"}

	if _, err := newFindMRsRequest(in, "https://gitlab.example.com"); err == nil {
		t.Error("newFindMRsRequest() error = nil, want an error for an unsupported format")
	}
}

// TestNewGetCommentsRequest_mapsEveryField is the acceptance check for
// TZ.md section 7.3 on the get_comments tool.
func TestNewGetCommentsRequest_mapsEveryField(t *testing.T) {
	in := mcptool.GetCommentsInput{
		User:            "alice",
		From:            "2026-01-01",
		To:              "2026-06-30",
		FromArtifact:    "artifacts/mr_list_abc.yaml",
		ArtifactsDir:    "out",
		Format:          "yaml",
		Refresh:         false,
		CacheTTLSeconds: 0,
	}

	req, err := newGetCommentsRequest(in, "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("newGetCommentsRequest() error = %v", err)
	}

	if req.GitlabURL != "https://gitlab.example.com" {
		t.Errorf("GitlabURL = %q, want %q", req.GitlabURL, "https://gitlab.example.com")
	}
	if req.User != "alice" {
		t.Errorf("User = %q, want %q", req.User, "alice")
	}
	if req.FromArtifact != "artifacts/mr_list_abc.yaml" {
		t.Errorf("FromArtifact = %q, want %q", req.FromArtifact, "artifacts/mr_list_abc.yaml")
	}
	if req.Dir != "out" {
		t.Errorf("Dir = %q, want %q", req.Dir, "out")
	}
	if req.Format != artifact.FormatYAML {
		t.Errorf("Format = %q, want %q", req.Format, artifact.FormatYAML)
	}
}

// TestNewGetCommentsRequest_mapsExplicitMRs confirms the mrs field
// passes through as-is: it is already app.GetCommentsRequest.MRs' exact
// type (artifact.MRRef), so no adaptation happens here.
func TestNewGetCommentsRequest_mapsExplicitMRs(t *testing.T) {
	in := mcptool.GetCommentsInput{
		User: "1", From: "2026-01-01", To: "2026-01-02",
		MRs: []artifact.MRRef{{ProjectID: 123, MRIID: 77}},
	}

	req, err := newGetCommentsRequest(in, "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("newGetCommentsRequest() error = %v", err)
	}
	if len(req.MRs) != 1 || req.MRs[0].ProjectID != 123 || req.MRs[0].MRIID != 77 {
		t.Errorf("MRs = %+v, want [{123 77}]", req.MRs)
	}
}

// TestNewGetClassifyBatchRequest_mapsEveryField is the acceptance check
// for TZ.md section 7.3 on the get_classify_batch tool.
func TestNewGetClassifyBatchRequest_mapsEveryField(t *testing.T) {
	tax := classify.DefaultTaxonomy()
	in := mcptool.GetClassifyBatchInput{
		FromArtifact: "artifacts/comment_list_abc.yaml",
		Model:        "glm-5.2",
		Taxonomy:     &tax,
		ArtifactsDir: "out",
	}

	req := newGetClassifyBatchRequest(in)

	if req.CommentListPath != "artifacts/comment_list_abc.yaml" {
		t.Errorf("CommentListPath = %q, want %q", req.CommentListPath, "artifacts/comment_list_abc.yaml")
	}
	if req.Model != "glm-5.2" {
		t.Errorf("Model = %q, want %q", req.Model, "glm-5.2")
	}
	if req.Taxonomy == nil || req.Taxonomy.Version != tax.Version {
		t.Errorf("Taxonomy = %v, want a pointer to %v", req.Taxonomy, tax)
	}
	if req.Dir != "out" {
		t.Errorf("Dir = %q, want %q", req.Dir, "out")
	}
}

// TestNewSaveLabelsRequest_defaultsClassifiedAtToNow confirms an omitted
// classified_at falls back to the current time rather than a zero
// time.Time, which would fail artifact.WriteLabeledComments's mandatory
// classifier check (TZ.md section 4.3).
func TestNewSaveLabelsRequest_defaultsClassifiedAtToNow(t *testing.T) {
	before := time.Now()
	req, err := newSaveLabelsRequest(mcptool.SaveLabelsInput{
		FromArtifact: "artifacts/comment_list_abc.yaml",
		Labels:       []classify.NoteLabel{{NoteID: 1, Label: "bug"}},
		Tool:         "opencode",
		Model:        "glm-5.2",
	})
	after := time.Now()
	if err != nil {
		t.Fatalf("newSaveLabelsRequest() error = %v", err)
	}
	if req.ClassifiedAt.Before(before) || req.ClassifiedAt.After(after) {
		t.Errorf("ClassifiedAt = %v, want between %v and %v", req.ClassifiedAt, before, after)
	}
}

// TestNewSaveLabelsRequest_parsesExplicitClassifiedAt confirms an
// explicit RFC 3339 classified_at is parsed and used as-is.
func TestNewSaveLabelsRequest_parsesExplicitClassifiedAt(t *testing.T) {
	req, err := newSaveLabelsRequest(mcptool.SaveLabelsInput{
		FromArtifact: "artifacts/comment_list_abc.yaml",
		Labels:       []classify.NoteLabel{{NoteID: 1, Label: "bug"}},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: "2026-07-15T16:40:00Z",
	})
	if err != nil {
		t.Fatalf("newSaveLabelsRequest() error = %v", err)
	}
	want, _ := time.Parse(time.RFC3339, "2026-07-15T16:40:00Z")
	if !req.ClassifiedAt.Equal(want) {
		t.Errorf("ClassifiedAt = %v, want %v", req.ClassifiedAt, want)
	}
}

// TestNewSaveLabelsRequest_rejectsBadClassifiedAt confirms a malformed
// classified_at is rejected before app.SaveLabels ever runs.
func TestNewSaveLabelsRequest_rejectsBadClassifiedAt(t *testing.T) {
	_, err := newSaveLabelsRequest(mcptool.SaveLabelsInput{
		FromArtifact: "artifacts/comment_list_abc.yaml",
		Labels:       []classify.NoteLabel{{NoteID: 1, Label: "bug"}},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: "not-a-timestamp",
	})
	if err == nil {
		t.Error("newSaveLabelsRequest() error = nil, want an error for a malformed classified_at")
	}
}

// TestNewFilterCommentsRequest_mapsEveryField is the acceptance check
// for TZ.md section 7.3 on the filter_comments tool.
func TestNewFilterCommentsRequest_mapsEveryField(t *testing.T) {
	in := mcptool.FilterCommentsInput{
		FromArtifact: "artifacts/labeled_comments_abc.yaml",
		Labels:       []string{"bug", "style"},
		Group:        "my-group",
		Project:      "my-group/repo",
		ArtifactsDir: "out",
		Format:       "json",
	}
	from, _ := domain.ParseDate("2026-01-01")
	to, _ := domain.ParseDate("2026-06-30")
	projectID := int64(123)

	req, err := newFilterCommentsRequest(in, &from, &to, []int64{1, 2, 3}, &projectID)
	if err != nil {
		t.Fatalf("newFilterCommentsRequest() error = %v", err)
	}

	if req.LabeledCommentsPath != "artifacts/labeled_comments_abc.yaml" {
		t.Errorf("LabeledCommentsPath = %q, want %q", req.LabeledCommentsPath, "artifacts/labeled_comments_abc.yaml")
	}
	if len(req.Labels) != 2 || req.Labels[0] != "bug" || req.Labels[1] != "style" {
		t.Errorf("Labels = %v, want [bug style]", req.Labels)
	}
	if req.From == nil || *req.From != from || req.To == nil || *req.To != to {
		t.Errorf("From/To = %v/%v, want %v/%v", req.From, req.To, from, to)
	}
	if req.Group != "my-group" || req.Project != "my-group/repo" {
		t.Errorf("Group/Project = %q/%q, want %q/%q", req.Group, req.Project, "my-group", "my-group/repo")
	}
	if len(req.ProjectIDs) != 3 {
		t.Errorf("ProjectIDs = %v, want 3 entries", req.ProjectIDs)
	}
	if req.ProjectID == nil || *req.ProjectID != 123 {
		t.Errorf("ProjectID = %v, want pointer to 123", req.ProjectID)
	}
	if req.Format != artifact.FormatJSON {
		t.Errorf("Format = %q, want %q", req.Format, artifact.FormatJSON)
	}
}

// TestNewGetStatsRequest_mapsEveryField is the acceptance check for
// TZ.md section 7.3 on the get_stats tool.
func TestNewGetStatsRequest_mapsEveryField(t *testing.T) {
	req, err := newGetStatsRequest(mcptool.GetStatsInput{
		ArtifactPath: "artifacts/mr_list_abc.yaml",
		ArtifactsDir: "out",
		Format:       "yaml",
	})
	if err != nil {
		t.Fatalf("newGetStatsRequest() error = %v", err)
	}
	if req.ArtifactPath != "artifacts/mr_list_abc.yaml" {
		t.Errorf("ArtifactPath = %q, want %q", req.ArtifactPath, "artifacts/mr_list_abc.yaml")
	}
	if req.Dir != "out" {
		t.Errorf("Dir = %q, want %q", req.Dir, "out")
	}
	if req.Format != artifact.FormatYAML {
		t.Errorf("Format = %q, want %q", req.Format, artifact.FormatYAML)
	}
}

// TestNewGetStatsRequest_rejectsBadFormat confirms an unknown format
// value is rejected before an app.GetStatsRequest is even built.
func TestNewGetStatsRequest_rejectsBadFormat(t *testing.T) {
	if _, err := newGetStatsRequest(mcptool.GetStatsInput{ArtifactPath: "x", Format: "csv"}); err == nil {
		t.Error("newGetStatsRequest() error = nil, want an error for an unsupported format")
	}
}
