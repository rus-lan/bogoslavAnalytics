package app

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// credentialedGitlabURL mirrors the auditor's proven A2 case: GITLAB_URL
// with a username and password embedded, GitLab's own real
// "oauth2:token@host" idiom.
const credentialedGitlabURL = "http://gluser:URLCANARY-99999@127.0.0.1:44297"

// wantSanitizedGitlabURL is credentialedGitlabURL with the userinfo
// stripped -- what every artifact/cache-key use of the URL must show
// instead.
const wantSanitizedGitlabURL = "http://127.0.0.1:44297"

// canary is the exact secret substring that must never appear anywhere
// in a written artifact's bytes, regardless of format.
const canary = "URLCANARY-99999"

var allFormats = []artifact.Format{artifact.FormatYAML, artifact.FormatJSON, artifact.FormatText, artifact.FormatHTML}

// TestFindMRs_urlCredentialsNeverReachTheWrittenArtifact is the A2 proof
// for mr_list: source.gitlab_url and query.gitlab_url must both be the
// sanitized URL, and the raw file bytes -- in every one of the four
// formats -- must never contain the embedded password, not even in the
// write-only text/html renderings that echo source.gitlab_url a second
// time.
func TestFindMRs_urlCredentialsNeverReachTheWrittenArtifact(t *testing.T) {
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	summaries := []gitlab.MergeRequestSummary{
		{MergeRequest: domain.MergeRequest{ProjectID: 1, IID: 7, ProjectPath: "g/p", Title: "t", WebURL: "u", CreatedAt: at, UpdatedAt: at}, UserNotesCount: 10},
	}

	for _, format := range allFormats {
		t.Run(string(format), func(t *testing.T) {
			dir := t.TempDir()
			client := &fakeClient{
				mergeRequestsFn: func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
					return summaries, nil
				},
				discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
					return notesFrom(42, 4, at), nil
				},
			}

			req := FindMRsRequest{
				GitlabURL: credentialedGitlabURL,
				User:      "42",
				From:      domain.NewDate(2026, time.January, 1),
				To:        domain.NewDate(2026, time.June, 30),
				MoreThan:  0,
				Strict:    true,
				Dir:       dir,
				Format:    format,
				Now:       func() time.Time { return now },
			}

			result, err := FindMRs(context.Background(), client, req)
			if err != nil {
				t.Fatalf("FindMRs() error = %v", err)
			}

			if result.Doc.Source.GitlabURL != wantSanitizedGitlabURL {
				t.Errorf("Doc.Source.GitlabURL = %q, want %q", result.Doc.Source.GitlabURL, wantSanitizedGitlabURL)
			}
			if result.Doc.Query.GitlabURL != wantSanitizedGitlabURL {
				t.Errorf("Doc.Query.GitlabURL = %q, want %q", result.Doc.Query.GitlabURL, wantSanitizedGitlabURL)
			}

			raw, err := os.ReadFile(result.Path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", result.Path, err)
			}
			if bytes.Contains(raw, []byte(canary)) {
				t.Errorf("written artifact %q (%s) contains the credential canary %q:\n%s", result.Path, format, canary, raw)
			}
			if bytes.Contains(raw, []byte("gluser")) {
				t.Errorf("written artifact %q (%s) contains the username %q:\n%s", result.Path, format, "gluser", raw)
			}
		})
	}
}

// TestGetComments_urlCredentialsNeverReachTheWrittenArtifact is the same
// A2 proof for comment_list: CommentQuery carries no gitlab_url field
// at all, so only source.gitlab_url is at stake here, but it must still
// be sanitized in every format, including the write-only renderings.
func TestGetComments_urlCredentialsNeverReachTheWrittenArtifact(t *testing.T) {
	at := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	for _, format := range allFormats {
		t.Run(string(format), func(t *testing.T) {
			dir := t.TempDir()
			client := &fakeDiscussionsClient{
				discussionsFn: func(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error) {
					return []domain.Discussion{discussion("d", note(1, 42, false, at))}, nil
				},
			}

			req := GetCommentsRequest{
				GitlabURL: credentialedGitlabURL,
				User:      "42",
				From:      domain.NewDate(2026, time.January, 1),
				To:        domain.NewDate(2026, time.June, 30),
				MRs:       []artifact.MRRef{{ProjectID: 1, MRIID: 7}},
				Dir:       dir,
				Format:    format,
				Now:       func() time.Time { return now },
			}

			result, err := GetComments(context.Background(), client, req)
			if err != nil {
				t.Fatalf("GetComments() error = %v", err)
			}

			if result.Doc.Source.GitlabURL != wantSanitizedGitlabURL {
				t.Errorf("Doc.Source.GitlabURL = %q, want %q", result.Doc.Source.GitlabURL, wantSanitizedGitlabURL)
			}

			raw, err := os.ReadFile(result.Path)
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", result.Path, err)
			}
			if bytes.Contains(raw, []byte(canary)) {
				t.Errorf("written artifact %q (%s) contains the credential canary %q:\n%s", result.Path, format, canary, raw)
			}
			if bytes.Contains(raw, []byte("gluser")) {
				t.Errorf("written artifact %q (%s) contains the username %q:\n%s", result.Path, format, "gluser", raw)
			}
		})
	}
}
