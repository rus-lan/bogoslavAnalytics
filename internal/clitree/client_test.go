package clitree

import (
	"errors"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// TestNewGitlabClient_missingTokenReturnsClearError proves a missing
// GITLAB_TOKEN produces a clear, wrapped error instead of a panic
// somewhere downstream when a nil client would otherwise reach an app
// function.
func TestNewGitlabClient_missingTokenReturnsClearError(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_URL", "")

	client, err := newGitlabClient()
	if err == nil {
		t.Fatal("newGitlabClient() error = nil, want a missing-token error")
	}
	if !errors.Is(err, gitlab.ErrMissingToken) {
		t.Errorf("newGitlabClient() error = %v, want errors.Is(err, gitlab.ErrMissingToken)", err)
	}
	if client != nil {
		t.Errorf("newGitlabClient() client = %v, want nil", client)
	}
}

func TestNewGitlabClient_tokenPresentSucceeds(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "dummy-token")
	t.Setenv("GITLAB_URL", "")

	client, err := newGitlabClient()
	if err != nil {
		t.Fatalf("newGitlabClient() error = %v, want nil", err)
	}
	if client == nil {
		t.Fatal("newGitlabClient() client = nil, want non-nil")
	}
}

func TestResolvedGitlabURL(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "unset falls back to gitlab.com", env: "", want: "https://gitlab.com"},
		{name: "set is used as-is", env: "https://gitlab.example.com", want: "https://gitlab.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_URL", tt.env)
			if got := resolvedGitlabURL(); got != tt.want {
				t.Errorf("resolvedGitlabURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseNumericID(t *testing.T) {
	tests := []struct {
		value   string
		wantN   int64
		wantOK  bool
		comment string
	}{
		{value: "42", wantN: 42, wantOK: true, comment: "digits only"},
		{value: "0", wantN: 0, wantOK: true, comment: "zero is still all-digits"},
		{value: "my-group/repo", wantN: 0, wantOK: false, comment: "path"},
		{value: "", wantN: 0, wantOK: false, comment: "empty"},
		{value: "-5", wantN: 0, wantOK: false, comment: "leading sign is not a plain digit run"},
	}
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			n, ok := parseNumericID(tt.value)
			if n != tt.wantN || ok != tt.wantOK {
				t.Errorf("parseNumericID(%q) = (%d, %v), want (%d, %v)", tt.value, n, ok, tt.wantN, tt.wantOK)
			}
		})
	}
}

func TestBuildGitlabID(t *testing.T) {
	if got := buildGitlabID("42"); got.String() != "42" {
		t.Errorf("buildGitlabID(%q).String() = %q, want %q", "42", got.String(), "42")
	}
	if got := buildGitlabID("my-group/repo"); got.String() != "my-group/repo" {
		t.Errorf("buildGitlabID(%q).String() = %q, want %q", "my-group/repo", got.String(), "my-group/repo")
	}
}
