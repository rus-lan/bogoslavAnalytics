package main

import (
	"errors"
	"testing"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// TestNewGitlabClientFromEnv_missingTokenIsAClearError is the acceptance
// check for TZ.md section 2.5: a missing GITLAB_TOKEN must surface as a
// clear, wrapped error -- never a panic, and never a nil client silently
// handed to a tool handler.
func TestNewGitlabClientFromEnv_missingTokenIsAClearError(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_URL", "")

	client, err := newGitlabClientFromEnv()

	if err == nil {
		t.Fatal("newGitlabClientFromEnv() error = nil, want an error when GITLAB_TOKEN is unset")
	}
	if !errors.Is(err, gitlab.ErrMissingToken) {
		t.Errorf("newGitlabClientFromEnv() error = %v, want it to wrap gitlab.ErrMissingToken", err)
	}
	if client != nil {
		t.Errorf("newGitlabClientFromEnv() client = %v, want nil on error", client)
	}
}

// TestRun_missingTokenReturnsErrorNotPanic exercises the same case
// through run itself (main's own entry point, minus os.Exit), confirming
// the whole startup path returns cleanly instead of panicking when
// GITLAB_TOKEN is unset.
func TestRun_missingTokenReturnsErrorNotPanic(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_URL", "")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("run() panicked: %v", r)
		}
	}()

	if err := run(t.Context(), testLogger()); err == nil {
		t.Fatal("run() error = nil, want an error when GITLAB_TOKEN is unset")
	}
}
