package clitree

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in      string
		want    artifact.Format
		wantErr bool
	}{
		{in: "json", want: artifact.FormatJSON},
		{in: "yaml", want: artifact.FormatYAML},
		{in: "text", want: artifact.FormatText},
		{in: "html", want: artifact.FormatHTML},
		{in: "xml", wantErr: true},
		{in: "", wantErr: true},
		{in: "JSON", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseFormat(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFormat(%q) error = nil, want an error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFormat(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseFormat(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCommonOutputFlags_formatHonestPerCommand checks that --format's
// help text on each command names exactly the wire formats that command
// accepts: get-stats and get-classify-batch reject text/html (their
// result is not one of the four artifact kinds), so their --format help
// must not mention either word, while the four commands that do write
// one of the four artifact kinds must keep the full, accurate list.
func TestCommonOutputFlags_formatHonestPerCommand(t *testing.T) {
	tests := []struct {
		name          string
		newCmd        func() *cobra.Command
		acceptsFourth bool // accepts text and html, on top of json/yaml
	}{
		{name: "find-mrs", newCmd: newFindMRsCmd, acceptsFourth: true},
		{name: "get-comments", newCmd: newGetCommentsCmd, acceptsFourth: true},
		{name: "filter-comments", newCmd: newFilterCommentsCmd, acceptsFourth: true},
		{name: "save-labels", newCmd: newSaveLabelsCmd, acceptsFourth: true},
		{name: "get-stats", newCmd: newGetStatsCmd, acceptsFourth: false},
		{name: "get-classify-batch", newCmd: newGetClassifyBatchCmd, acceptsFourth: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.newCmd().Flags().Lookup("format")
			if flag == nil {
				t.Fatal("--format flag not registered")
			}

			hasHTML := strings.Contains(flag.Usage, "html")
			hasText := strings.Contains(flag.Usage, "text")
			if hasHTML != tt.acceptsFourth || hasText != tt.acceptsFourth {
				t.Errorf("--format usage = %q, mentions html=%v text=%v, want both %v",
					flag.Usage, hasHTML, hasText, tt.acceptsFourth)
			}
			if !tt.acceptsFourth && !strings.Contains(flag.Usage, "json or yaml") {
				t.Errorf("--format usage = %q, want it to say json or yaml", flag.Usage)
			}
		})
	}
}

// TestCommonOutputFlags_artifactsDirHonestPerCommand checks that
// --artifacts-dir's help text only claims a cache lookup on the
// commands that actually consult one (find-mrs, get-comments,
// get-classify-batch): filter-comments, save-labels and get-stats never
// consult a cache, so their --artifacts-dir help must not claim one.
func TestCommonOutputFlags_artifactsDirHonestPerCommand(t *testing.T) {
	tests := []struct {
		name   string
		newCmd func() *cobra.Command
		caches bool
	}{
		{name: "find-mrs", newCmd: newFindMRsCmd, caches: true},
		{name: "get-comments", newCmd: newGetCommentsCmd, caches: true},
		{name: "get-classify-batch", newCmd: newGetClassifyBatchCmd, caches: true},
		{name: "filter-comments", newCmd: newFilterCommentsCmd, caches: false},
		{name: "save-labels", newCmd: newSaveLabelsCmd, caches: false},
		{name: "get-stats", newCmd: newGetStatsCmd, caches: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.newCmd().Flags().Lookup("artifacts-dir")
			if flag == nil {
				t.Fatal("--artifacts-dir flag not registered")
			}

			hasCache := strings.Contains(flag.Usage, "cache")
			if hasCache != tt.caches {
				t.Errorf("--artifacts-dir usage = %q, mentions cache=%v, want %v", flag.Usage, hasCache, tt.caches)
			}
		})
	}
}

// TestArtifactsDirBehaviorHonestPerCommand checks that --artifacts-dir's
// help text on each command matches exactly what that command does with
// the directory: whether a result ever lands under it (written-to) and
// whether an existing artifact is ever looked up there first (read-from,
// as a cache). Every command falls into one of four combinations:
// find-mrs and get-comments do both; filter-comments, save-labels and
// get-stats only ever write; get-classify-batch only ever reads -- it
// never writes anything under this directory itself (TZ.md section 8.1:
// save-labels writes artifact-3, not get-classify-batch), so its help
// must say so plainly instead of claiming a write that never happens.
func TestArtifactsDirBehaviorHonestPerCommand(t *testing.T) {
	tests := []struct {
		name       string
		newCmd     func() *cobra.Command
		writesHere bool // usage claims a result is written under the directory
		readsHere  bool // usage claims an existing artifact is looked up there first, as a cache
	}{
		{name: "find-mrs", newCmd: newFindMRsCmd, writesHere: true, readsHere: true},
		{name: "get-comments", newCmd: newGetCommentsCmd, writesHere: true, readsHere: true},
		{name: "filter-comments", newCmd: newFilterCommentsCmd, writesHere: true, readsHere: false},
		{name: "save-labels", newCmd: newSaveLabelsCmd, writesHere: true, readsHere: false},
		{name: "get-stats", newCmd: newGetStatsCmd, writesHere: true, readsHere: false},
		{name: "get-classify-batch", newCmd: newGetClassifyBatchCmd, writesHere: false, readsHere: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.newCmd().Flags().Lookup("artifacts-dir")
			if flag == nil {
				t.Fatal("--artifacts-dir flag not registered")
			}

			claimsWrite := strings.Contains(flag.Usage, "is written under")
			if claimsWrite != tt.writesHere {
				t.Errorf("--artifacts-dir usage = %q, claims a write = %v, want %v", flag.Usage, claimsWrite, tt.writesHere)
			}

			claimsLookup := strings.Contains(flag.Usage, "looked up")
			if claimsLookup != tt.readsHere {
				t.Errorf("--artifacts-dir usage = %q, claims a cache lookup = %v, want %v", flag.Usage, claimsLookup, tt.readsHere)
			}

			if !tt.writesHere && !strings.Contains(flag.Usage, "never writes") {
				t.Errorf("--artifacts-dir usage = %q, want it to say plainly that this command never writes under this directory", flag.Usage)
			}
		})
	}
}

// TestFindMRs_moreThanUsageStaysStrict guards the one help string TZ.md
// requires verbatim (TZ.md sections 1.2, 7.2): --more-than N must keep
// saying STRICTLY more than N, not "at least N" or "N or more".
func TestFindMRs_moreThanUsageStaysStrict(t *testing.T) {
	flag := newFindMRsCmd().Flags().Lookup("more-than")
	if flag == nil {
		t.Fatal("--more-than flag not registered")
	}
	if !strings.Contains(flag.Usage, "STRICTLY more than") {
		t.Errorf("--more-than usage = %q, want it to say STRICTLY more than", flag.Usage)
	}
}
