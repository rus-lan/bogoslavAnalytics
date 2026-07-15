package main

import (
	"fmt"
	"os"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

// defaultGitlabURL is used to stamp source.gitlab_url on a freshly
// written artifact when GITLAB_URL is not set (TZ.md section 2.5),
// mirroring gitlab.NewClientFromEnv's own default so the value recorded
// on the artifact always matches the instance the client actually talked
// to. This mirrors bogoslav-cli's identical constant: the two binaries
// are separate package main trees and cannot share it directly.
const defaultGitlabURL = "https://gitlab.com"

// newGitlabClientFromEnv builds a GitLab client from GITLAB_URL/
// GITLAB_TOKEN (TZ.md section 2.5), wrapping a missing token in a clear,
// startup-level error instead of letting a nil client reach a tool
// handler and panic.
func newGitlabClientFromEnv() (*gitlab.Client, error) {
	client, err := gitlab.NewClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("connect to GitLab: %w", err)
	}
	return client, nil
}

// resolvedGitlabURL returns GITLAB_URL, or defaultGitlabURL when it is
// not set, for the request fields (FindMRsRequest.GitlabURL,
// GetCommentsRequest.GitlabURL) that record the instance a fresh fetch
// talked to.
func resolvedGitlabURL() string {
	if url := os.Getenv("GITLAB_URL"); url != "" {
		return url
	}
	return defaultGitlabURL
}
