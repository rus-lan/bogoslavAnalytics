package main

import (
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func TestNewFindMRsRequest_mapsFlagsToRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want app.FindMRsRequest
	}{
		{
			name: "minimal required flags",
			args: []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30"},
			want: app.FindMRsRequest{
				GitlabURL: "https://gitlab.example.com",
				User:      "42",
				From:      domain.NewDate(2026, time.January, 1),
				To:        domain.NewDate(2026, time.June, 30),
				Format:    artifact.FormatYAML,
			},
		},
		{
			name: "every flag set",
			args: []string{
				"--user", "alice",
				"--from", "2026-01-01",
				"--to", "2026-06-30",
				"--more-than", "5",
				"--group", "my-group",
				"--project", "my-group/repo",
				"--strict",
				"--format", "json",
				"--artifacts-dir", "out",
				"--refresh",
				"--cache-ttl", "1h",
			},
			want: app.FindMRsRequest{
				GitlabURL: "https://gitlab.example.com",
				User:      "alice",
				From:      domain.NewDate(2026, time.January, 1),
				To:        domain.NewDate(2026, time.June, 30),
				MoreThan:  5,
				Group:     "my-group",
				Project:   "my-group/repo",
				Strict:    true,
				Dir:       "out",
				Format:    artifact.FormatJSON,
				Cache:     app.CacheOptions{TTL: time.Hour, Refresh: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flags := buildFlags(t, registerFindMRsFlags, tt.args)
			got, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
			if err != nil {
				t.Fatalf("newFindMRsRequest() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newFindMRsRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewFindMRsRequest_mrPointerOnlySetWhenFlagChanged(t *testing.T) {
	t.Run("mr not passed leaves MR nil", func(t *testing.T) {
		cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30"})
		got, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
		if err != nil {
			t.Fatalf("newFindMRsRequest() error = %v", err)
		}
		if got.MR != nil {
			t.Errorf("MR = %v, want nil", *got.MR)
		}
	})

	t.Run("mr passed as 0 is still set", func(t *testing.T) {
		cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--project", "g/p", "--mr", "0"})
		got, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
		if err != nil {
			t.Fatalf("newFindMRsRequest() error = %v", err)
		}
		if got.MR == nil || *got.MR != 0 {
			t.Errorf("MR = %v, want pointer to 0", got.MR)
		}
	})

	t.Run("mr passed as 9 is set", func(t *testing.T) {
		cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--project", "g/p", "--mr", "9"})
		got, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
		if err != nil {
			t.Fatalf("newFindMRsRequest() error = %v", err)
		}
		if got.MR == nil || *got.MR != 9 {
			t.Errorf("MR = %v, want pointer to 9", got.MR)
		}
	})
}

func TestNewFindMRsRequest_rejectsUnknownFormat(t *testing.T) {
	cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--format", "xml"})
	_, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
	if err == nil {
		t.Fatal("newFindMRsRequest() error = nil, want an error for --format xml")
	}
}

func TestNewFindMRsRequest_rejectsUnparsableDate(t *testing.T) {
	cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "not-a-date", "--to", "2026-06-30"})
	_, err := newFindMRsRequest(cmd, *flags, "https://gitlab.example.com")
	if err == nil {
		t.Fatal("newFindMRsRequest() error = nil, want an error for --from not-a-date")
	}
}

// TestFindMRs_mrWithoutProjectFailsWithClearError proves the command
// surfaces app.ErrPointModeRequiresProject rather than papering over it,
// and does so without duplicating the check itself (TZ.md sections 1.2,
// 7.2): app.FindMRs returns this error before ever touching its client
// argument, so a dummy (never-dialed) GITLAB_TOKEN is enough to exercise
// the whole command end to end here, with zero network calls.
func TestFindMRs_mrWithoutProjectFailsWithClearError(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "dummy-token")
	t.Setenv("GITLAB_URL", "")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"find-mrs", "--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--mr", "9"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want ErrPointModeRequiresProject")
	}
	if !errors.Is(err, app.ErrPointModeRequiresProject) {
		t.Errorf("Execute() error = %v, want errors.Is(err, app.ErrPointModeRequiresProject)", err)
	}
}
