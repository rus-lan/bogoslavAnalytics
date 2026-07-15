package main

import (
	"strconv"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// parseNumericID reports whether value is made only of decimal digits,
// returning its numeric value when it is. It mirrors the identical rule
// internal/app's own (unexported) parseNumericID and
// bogoslav-cli's own copy apply to user, group and project (TZ.md
// sections 5.0 and 14, item 1): a value made only of digits is a numeric
// id, used as-is; anything else is a namespaced path, resolved through
// GitLab when a tool needs a numeric id out of it.
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

// buildGitlabID converts a group/project value into a gitlab.ID the same
// way parseNumericID classifies it: an all-digits value becomes a
// gitlab.NumericID, anything else a gitlab.PathID.
func buildGitlabID(value string) gitlab.ID {
	if n, ok := parseNumericID(value); ok {
		return gitlab.NumericID(n)
	}
	return gitlab.PathID(value)
}
