package clitree

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// defaultGitlabURL is used to stamp source.gitlab_url on a freshly
// written artifact when GITLAB_URL is not set (TZ.md section 2.5),
// mirroring gitlab.NewClientFromEnv's own default so the value recorded
// on the artifact always matches the instance the client actually talked
// to.
const defaultGitlabURL = "https://gitlab.com"

// newGitlabClient builds a GitLab client from GITLAB_URL/GITLAB_TOKEN/
// BOGOSLAV_TIMEOUT (TZ.md section 2.5), wrapping a missing token in a
// clear, command-level error instead of letting a nil client reach an
// app function and panic. opts is normally either empty (no --timeout
// override; BOGOSLAV_TIMEOUT or gitlab.DefaultTimeout wins) or exactly
// one gitlab.WithTimeout(...), built by timeoutOption from the command's
// own --timeout flag.
func newGitlabClient(opts ...gitlab.Option) (*gitlab.Client, error) {
	client, err := gitlab.NewClientFromEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to GitLab: %w", err)
	}
	return client, nil
}

// timeoutOption returns a gitlab.WithTimeout option built from --timeout,
// but only when the flag was actually passed: cmd.Flags().Changed keeps
// an unset --timeout from silently overriding BOGOSLAV_TIMEOUT (or
// gitlab.DefaultTimeout, if that is unset too) with the flag's own zero
// value, which would otherwise be indistinguishable from an explicit
// "--timeout 0s" (disable the deadline entirely).
func timeoutOption(cmd *cobra.Command, timeout time.Duration) ([]gitlab.Option, error) {
	if !cmd.Flags().Changed("timeout") {
		return nil, nil
	}
	if err := gitlab.ValidateTimeout(timeout); err != nil {
		return nil, fmt.Errorf("--timeout: %w", err)
	}
	return []gitlab.Option{gitlab.WithTimeout(timeout)}, nil
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

// parseNumericID reports whether value is made only of decimal digits,
// returning its numeric value when it is. It mirrors the identical rule
// internal/app's own (unexported) parseNumericID applies to --user,
// --group and --project (TZ.md sections 5.0 and 14, item 1): a --group
// or --project value made only of digits is a numeric id, used as-is;
// anything else is a namespaced path, resolved through GitLab when this
// command needs a numeric id out of it.
func parseNumericID(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// buildGitlabID converts a --group/--project value into a gitlab.ID the
// same way parseNumericID classifies it: an all-digits value becomes a
// gitlab.NumericID, anything else a gitlab.PathID.
func buildGitlabID(value string) gitlab.ID {
	if n, ok := parseNumericID(value); ok {
		return gitlab.NumericID(n)
	}
	return gitlab.PathID(value)
}
