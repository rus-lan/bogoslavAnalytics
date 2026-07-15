package clitree

import (
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

func TestNewGetCommentsRequest_mapsFlagsToRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want app.GetCommentsRequest
	}{
		{
			name: "from-artifact",
			args: []string{"--user", "alice", "--from", "2026-01-01", "--to", "2026-06-30", "--from-artifact", "artifacts/mr_list_x.yaml"},
			want: app.GetCommentsRequest{
				GitlabURL:    "https://gitlab.example.com",
				User:         "alice",
				From:         domain.NewDate(2026, time.January, 1),
				To:           domain.NewDate(2026, time.June, 30),
				FromArtifact: "artifacts/mr_list_x.yaml",
				Format:       artifact.FormatYAML,
			},
		},
		{
			name: "explicit project and repeated mr",
			args: []string{"--user", "alice", "--from", "2026-01-01", "--to", "2026-06-30", "--project", "123", "--mr", "7", "--mr", "9"},
			want: app.GetCommentsRequest{
				GitlabURL: "https://gitlab.example.com",
				User:      "alice",
				From:      domain.NewDate(2026, time.January, 1),
				To:        domain.NewDate(2026, time.June, 30),
				MRs: []artifact.MRRef{
					{ProjectID: 123, MRIID: 7},
					{ProjectID: 123, MRIID: 9},
				},
				Format: artifact.FormatYAML,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flags := buildFlags(t, registerGetCommentsFlags, tt.args)
			got, err := newGetCommentsRequest(cmd, *flags, "https://gitlab.example.com")
			if err != nil {
				t.Fatalf("newGetCommentsRequest() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newGetCommentsRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildMRRefs_mrWithoutProjectFailsWithClearError(t *testing.T) {
	cmd, flags := buildFlags(t, registerGetCommentsFlags, []string{
		"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--mr", "9",
	})
	_, err := buildMRRefs(cmd, flags.project, flags.mrs)
	if err == nil {
		t.Fatal("buildMRRefs() error = nil, want an error: --mr without --project")
	}
}

func TestBuildMRRefs_noMRsReturnsNil(t *testing.T) {
	cmd, flags := buildFlags(t, registerGetCommentsFlags, []string{
		"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--from-artifact", "x.yaml",
	})
	refs, err := buildMRRefs(cmd, flags.project, flags.mrs)
	if err != nil {
		t.Fatalf("buildMRRefs() error = %v", err)
	}
	if refs != nil {
		t.Errorf("buildMRRefs() = %v, want nil", refs)
	}
}

func TestNewGetCommentsRequest_rejectsUnknownFormat(t *testing.T) {
	cmd, flags := buildFlags(t, registerGetCommentsFlags, []string{
		"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--format", "xml",
	})
	_, err := newGetCommentsRequest(cmd, *flags, "https://gitlab.example.com")
	if err == nil {
		t.Fatal("newGetCommentsRequest() error = nil, want an error for --format xml")
	}
}
