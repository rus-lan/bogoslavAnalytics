package clitree

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// TestAddTimeoutFlag_onlyOnCommandsThatBuildAGitlabClient proves --timeout
// is registered exactly on find-mrs, get-comments and filter-comments --
// the three commands that ever call newGitlabClient -- and nowhere else,
// mirroring TestCommonOutputFlags_artifactsDirHonestPerCommand's own
// reasoning: a flag that looks like it does something but never does is
// dishonest.
func TestAddTimeoutFlag_onlyOnCommandsThatBuildAGitlabClient(t *testing.T) {
	tests := []struct {
		name    string
		newCmd  func() *cobra.Command
		hasFlag bool
	}{
		{name: "find-mrs", newCmd: newFindMRsCmd, hasFlag: true},
		{name: "get-comments", newCmd: newGetCommentsCmd, hasFlag: true},
		{name: "filter-comments", newCmd: newFilterCommentsCmd, hasFlag: true},
		{name: "save-labels", newCmd: newSaveLabelsCmd, hasFlag: false},
		{name: "get-stats", newCmd: newGetStatsCmd, hasFlag: false},
		{name: "get-classify-batch", newCmd: newGetClassifyBatchCmd, hasFlag: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.newCmd().Flags().Lookup("timeout")
			if (flag != nil) != tt.hasFlag {
				t.Errorf("--timeout registered = %v, want %v", flag != nil, tt.hasFlag)
			}
		})
	}
}

// TestAddTimeoutFlag_usageNamesDefaultAndDisable proves --timeout's help
// text names both the documented default (gitlab.DefaultTimeout) and how
// to disable the deadline, so a user reading --help finds both without
// guessing.
func TestAddTimeoutFlag_usageNamesDefaultAndDisable(t *testing.T) {
	flag := newFindMRsCmd().Flags().Lookup("timeout")
	if flag == nil {
		t.Fatal("--timeout flag not registered")
	}
	if !strings.Contains(flag.Usage, gitlab.DefaultTimeout.String()) {
		t.Errorf("--timeout usage = %q, want it to name the default %s", flag.Usage, gitlab.DefaultTimeout)
	}
	if !strings.Contains(flag.Usage, "BOGOSLAV_TIMEOUT") {
		t.Errorf("--timeout usage = %q, want it to mention BOGOSLAV_TIMEOUT", flag.Usage)
	}
	if !strings.Contains(flag.Usage, `"0s" disables it`) {
		t.Errorf("--timeout usage = %q, want it to say 0s disables it entirely", flag.Usage)
	}
}

// TestTimeoutOption_unsetReturnsNoOverride proves that leaving --timeout
// unset builds no gitlab.Option at all, so BOGOSLAV_TIMEOUT (or
// gitlab.DefaultTimeout, if that is unset too) decides the deadline
// inside gitlab.NewClientFromEnv, exactly as it would with no --timeout
// flag existing at all.
func TestTimeoutOption_unsetReturnsNoOverride(t *testing.T) {
	cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30"})

	opts, err := timeoutOption(cmd, flags.timeout)
	if err != nil {
		t.Fatalf("timeoutOption() error = %v, want nil", err)
	}
	if len(opts) != 0 {
		t.Errorf("timeoutOption() = %d options, want 0 when --timeout is not passed", len(opts))
	}
}

// TestTimeoutOption_explicitValueBuildsOverride proves that passing
// --timeout -- including the explicit "0s disables it" case -- builds
// exactly one gitlab.Option to hand to newGitlabClient. Whether that
// option actually reaches the outgoing HTTP request is proved
// end-to-end by TestFindMRs_timeoutFlagAbortsSlowServer below, not here:
// gitlab.Client keeps the resulting http.Client.Timeout unexported, so
// this test only checks the flag -> options-slice mapping in isolation,
// exactly like buildFlags-based tests elsewhere in this package check a
// flag -> request-struct mapping without a real GitLab call.
func TestTimeoutOption_explicitValueBuildsOverride(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "positive value", args: []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--timeout", "45s"}},
		{name: "explicit zero disables", args: []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--timeout", "0s"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flags := buildFlags(t, registerFindMRsFlags, tt.args)

			opts, err := timeoutOption(cmd, flags.timeout)
			if err != nil {
				t.Fatalf("timeoutOption() error = %v, want nil", err)
			}
			if len(opts) != 1 {
				t.Fatalf("timeoutOption() = %d options, want exactly 1 when --timeout is passed", len(opts))
			}
		})
	}
}

// TestTimeoutOption_rejectsNegative proves a negative --timeout is a
// clear, immediate error, not a value silently handed to net/http.
func TestTimeoutOption_rejectsNegative(t *testing.T) {
	cmd, flags := buildFlags(t, registerFindMRsFlags, []string{"--user", "42", "--from", "2026-01-01", "--to", "2026-06-30", "--timeout", "-5s"})

	_, err := timeoutOption(cmd, flags.timeout)
	if err == nil {
		t.Fatal("timeoutOption() error = nil, want an error for a negative --timeout")
	}
}

// TestFindMRs_timeoutFlagAbortsSlowServer runs the real find-mrs command
// (point mode, the cheapest path to a real GitLab call: one merge
// request lookup plus one /discussions call) against a fake GitLab
// server whose /discussions handler sleeps past a short --timeout,
// proving --timeout plumbs all the way from the flag through
// newGitlabClient into the real outgoing HTTP request bogoslav-cli
// makes -- not just into an app.FindMRsRequest field nothing reads.
func TestFindMRs_timeoutFlagAbortsSlowServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/5/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": 1, "iid": 9, "project_id": 5,
			"title": "fix bug", "web_url": "https://gitlab.example.com/my-group/repo/-/merge_requests/9",
			"created_at": "2026-03-01T10:00:00Z", "updated_at": "2026-03-05T10:00:00Z",
			"references": {"full": "my-group/repo!9"},
			"user_notes_count": 3
		}]`))
	})
	mux.HandleFunc("/api/v4/projects/5/merge_requests/9/discussions", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("GITLAB_URL", server.URL)
	t.Setenv("GITLAB_TOKEN", "dummy-token")
	t.Setenv("BOGOSLAV_TIMEOUT", "")

	dir := t.TempDir()

	cmd := NewRootCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"find-mrs",
		"--user", "42",
		"--from", "2026-01-01",
		"--to", "2026-06-30",
		"--project", "5",
		"--mr", "9",
		"--artifacts-dir", dir,
		"--timeout", "20ms",
	})

	start := time.Now()
	err := cmd.Execute()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Execute() error = nil, want a timeout error from the slow /discussions call")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Execute() error = %v, want it to mention context deadline exceeded", err)
	}
	if elapsed >= 150*time.Millisecond {
		t.Errorf("Execute() took %v, want it to abort well before the server's 150ms sleep", elapsed)
	}
}

// TestFindMRs_bogoslavTimeoutEnvAbortsSlowServer is
// TestFindMRs_timeoutFlagAbortsSlowServer's env-only twin: with no
// --timeout flag at all, BOGOSLAV_TIMEOUT must reach the exact same
// outgoing request, proving the flag and the env var plumb through
// identically.
func TestFindMRs_bogoslavTimeoutEnvAbortsSlowServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/5/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": 1, "iid": 9, "project_id": 5,
			"title": "fix bug", "web_url": "https://gitlab.example.com/my-group/repo/-/merge_requests/9",
			"created_at": "2026-03-01T10:00:00Z", "updated_at": "2026-03-05T10:00:00Z",
			"references": {"full": "my-group/repo!9"},
			"user_notes_count": 3
		}]`))
	})
	mux.HandleFunc("/api/v4/projects/5/merge_requests/9/discussions", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("GITLAB_URL", server.URL)
	t.Setenv("GITLAB_TOKEN", "dummy-token")
	t.Setenv("BOGOSLAV_TIMEOUT", "20ms")

	dir := t.TempDir()

	cmd := NewRootCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"find-mrs",
		"--user", "42",
		"--from", "2026-01-01",
		"--to", "2026-06-30",
		"--project", "5",
		"--mr", "9",
		"--artifacts-dir", dir,
	})

	start := time.Now()
	err := cmd.Execute()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Execute() error = nil, want a timeout error from the slow /discussions call")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Execute() error = %v, want it to mention context deadline exceeded", err)
	}
	if elapsed >= 150*time.Millisecond {
		t.Errorf("Execute() took %v, want it to abort well before the server's 150ms sleep", elapsed)
	}
}
