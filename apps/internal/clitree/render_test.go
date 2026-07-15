package clitree

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
)

// TestReportFormatMismatch_warnsWhenCacheHitFormatDiffersFromRequested
// guards the fix for the reviewer's finding at clitree/render.go:33-36:
// on a cache hit, the existing artifact's format need not match --format
// (cache.Lookup only ever matches json or yaml), so a mismatch must be
// reported on stderr instead of silently handed out.
func TestReportFormatMismatch_warnsWhenCacheHitFormatDiffersFromRequested(t *testing.T) {
	tests := []struct {
		name      string
		requested artifact.Format
		path      string
		wantNote  bool
	}{
		{name: "text requested, yaml cache hit", requested: artifact.FormatText, path: "artifacts/mr_list_abc.yaml", wantNote: true},
		{name: "html requested, json cache hit", requested: artifact.FormatHTML, path: "artifacts/mr_list_abc.json", wantNote: true},
		{name: "json requested, yaml cache hit", requested: artifact.FormatJSON, path: "artifacts/mr_list_abc.yaml", wantNote: true},
		{name: "yaml requested, yaml cache hit", requested: artifact.FormatYAML, path: "artifacts/mr_list_abc.yaml", wantNote: false},
		{name: "json requested, json cache hit", requested: artifact.FormatJSON, path: "artifacts/mr_list_abc.json", wantNote: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			reportFormatMismatch(cmd, tt.requested, tt.path)

			gotNote := stderr.Len() > 0
			if gotNote != tt.wantNote {
				t.Errorf("reportFormatMismatch(%q, %q) stderr = %q, wantNote = %v", tt.requested, tt.path, stderr.String(), tt.wantNote)
			}
			if gotNote {
				if !strings.Contains(stderr.String(), string(tt.requested)) {
					t.Errorf("stderr = %q, want it to mention the requested format %q", stderr.String(), tt.requested)
				}
				if !strings.Contains(stderr.String(), tt.path) {
					t.Errorf("stderr = %q, want it to mention the cache-hit path %q", stderr.String(), tt.path)
				}
			}
		})
	}
}
