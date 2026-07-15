package clitree

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// sectionRefPattern catches "section 7", "section 9.3.2", etc., case
// insensitively, wherever it turns up in help text.
var sectionRefPattern = regexp.MustCompile(`(?i)\bsection\s+\d`)

// collectUserFacingStrings walks cmd and every one of its descendants,
// gathering every string an installed binary's --help (or a tool built
// from this tree, such as SKILL.md) actually shows a user: each command's
// Short and Long, and every flag's Usage.
func collectUserFacingStrings(cmd *cobra.Command) []string {
	var out []string
	out = append(out, cmd.Short, cmd.Long)
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		out = append(out, f.Usage)
	})
	for _, sub := range cmd.Commands() {
		out = append(out, collectUserFacingStrings(sub)...)
	}
	return out
}

// TestNewRootCmd_helpTextHasNoDanglingInternalReferences proves that
// bogoslav-cli's --help (and anything generated from this same tree, such
// as bogoslav-skills' SKILL.md) never points a reader at TZ.md, a TZ.md
// section number, or an internal/ package path: none of those resolve for
// someone who only has the installed binary.
func TestNewRootCmd_helpTextHasNoDanglingInternalReferences(t *testing.T) {
	for _, s := range collectUserFacingStrings(NewRootCmd()) {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "tz.md") {
			t.Errorf("user-facing string contains %q: %q", "TZ.md", s)
		}
		if sectionRefPattern.MatchString(s) {
			t.Errorf("user-facing string contains a %q reference: %q", "section N", s)
		}
		if strings.Contains(s, "internal/") {
			t.Errorf("user-facing string contains %q: %q", "internal/", s)
		}
	}
}

// TestNewRootCmd_helpTextKeepsProtectedContracts proves the command
// tree's help text still states the exact, deliberate contracts a user
// depends on, in words a plain string search can confirm.
func TestNewRootCmd_helpTextKeepsProtectedContracts(t *testing.T) {
	all := strings.Join(collectUserFacingStrings(NewRootCmd()), "\n")

	checks := []struct {
		name string
		want string
	}{
		{"--more-than is strict", "STRICTLY more than N comments by --user (exactly N is excluded)"},
		{"text/html are write-only and never a cache hit", "text and html are write-only: " +
			"they cannot be read back and cannot be passed to --from-artifact, and they are never a cache hit"},
		{"get-stats/get-classify-batch --format is json/yaml only", "output format: json or yaml"},
		{"get-classify-batch never writes under --artifacts-dir", "get-classify-batch never writes " +
			"anything under this directory itself"},
		{"get-stats writes under --artifacts-dir only when given", "without --artifacts-dir the " +
			"aggregate is only printed"},
		{"--refresh bypasses the cache", "bypass the cache and always call GitLab, even if a fresh cached artifact exists"},
	}

	for _, c := range checks {
		if !strings.Contains(all, c.want) {
			t.Errorf("%s: help text no longer contains %q", c.name, c.want)
		}
	}
}
