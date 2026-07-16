package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestWithTimeout_abortsSlowResponse proves WithTimeout actually reaches
// the outgoing request: a server that answers slower than the configured
// timeout must make the call fail, wrapping context.DeadlineExceeded
// (net/http's own documented behavior for http.Client.Timeout).
func TestWithTimeout_abortsSlowResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token", WithTimeout(20*time.Millisecond))

	start := time.Now()
	_, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("request() error = nil, want a timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("request() error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if elapsed >= 150*time.Millisecond {
		t.Errorf("request() took %v, want it to abort well before the server's 150ms sleep", elapsed)
	}
}

// TestWithTimeout_zeroDisablesDeadline proves WithTimeout(0) -- net/http's
// own documented meaning for http.Client.Timeout -- leaves a slow but
// finite response to succeed instead of being cut off.
func TestWithTimeout_zeroDisablesDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token", WithTimeout(0))

	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v, want nil (WithTimeout(0) must not cut off a slow-but-finite response)", err)
	}
	resp.Body.Close()
}

// TestNoWithTimeout_leavesNoClientLevelDeadline proves that never calling
// WithTimeout at all keeps this package's original behavior: no
// client-level deadline (only whatever the caller's own ctx enforces),
// so an existing caller of NewClient that never touches this option sees
// no behavior change.
func TestNoWithTimeout_leavesNoClientLevelDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if c.httpClient.Timeout != 0 {
		t.Fatalf("httpClient.Timeout = %v, want 0 (no WithTimeout call)", c.httpClient.Timeout)
	}

	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v, want nil", err)
	}
	resp.Body.Close()
}

// TestClient_request_ctxCancelAbortsInFlight proves ctx cancellation
// (what Ctrl-C via signal.NotifyContext ultimately triggers) still
// aborts an in-flight request promptly, independent of WithTimeout: the
// two mechanisms are additive, whichever fires first wins.
func TestClient_request_ctxCancelAbortsInFlight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// A generous WithTimeout, far longer than the cancellation below, so
	// this test only exercises ctx cancellation, not the Timeout.
	c := NewClient(srv.URL, "token", WithTimeout(10*time.Second))

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := c.request(ctx, http.MethodGet, "/users", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("request() error = nil, want a cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("request() error = %v, want it to wrap context.Canceled", err)
	}
	if elapsed >= 150*time.Millisecond {
		t.Errorf("request() took %v, want it to abort well before the server's 150ms sleep", elapsed)
	}
}

// TestDefaultTimeout_is2Minutes pins the documented default so it cannot
// silently drift: TZ.md section 2.5 and README.md both name this exact
// value.
func TestDefaultTimeout_is2Minutes(t *testing.T) {
	if DefaultTimeout != 2*time.Minute {
		t.Errorf("DefaultTimeout = %v, want 2m", DefaultTimeout)
	}
}

// TestNewClientFromEnv_defaultTimeoutAppliedWhenUnset proves a client
// built with no BOGOSLAV_TIMEOUT set carries DefaultTimeout, not no
// deadline at all -- the exact fix for the reported "requests hang with
// no timeout at all" gap.
func TestNewClientFromEnv_defaultTimeoutAppliedWhenUnset(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Errorf("httpClient.Timeout = %v, want DefaultTimeout (%v)", c.httpClient.Timeout, DefaultTimeout)
	}
}

// TestNewClientFromEnv_readsBogoslavTimeout proves BOGOSLAV_TIMEOUT
// overrides DefaultTimeout.
func TestNewClientFromEnv_readsBogoslavTimeout(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "45s")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.httpClient.Timeout != 45*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 45s", c.httpClient.Timeout)
	}
}

// TestNewClientFromEnv_bogoslavTimeoutZeroDisables proves BOGOSLAV_TIMEOUT=0
// disables the deadline entirely, for the user who really wants to wait.
func TestNewClientFromEnv_bogoslavTimeoutZeroDisables(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "0")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.httpClient.Timeout != 0 {
		t.Errorf("httpClient.Timeout = %v, want 0 (disabled)", c.httpClient.Timeout)
	}
}

// TestNewClientFromEnv_invalidBogoslavTimeoutRejected proves an
// unparsable BOGOSLAV_TIMEOUT is a clear, wrapped startup error, not a
// silently ignored value or a panic.
func TestNewClientFromEnv_invalidBogoslavTimeoutRejected(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "not-a-duration")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want an error for an unparsable BOGOSLAV_TIMEOUT")
	}
}

// TestNewClientFromEnv_negativeBogoslavTimeoutRejected proves a negative
// BOGOSLAV_TIMEOUT is rejected instead of silently producing undocumented
// http.Client.Timeout behavior.
func TestNewClientFromEnv_negativeBogoslavTimeoutRejected(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "-5s")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want an error for a negative BOGOSLAV_TIMEOUT")
	}
}

// TestNewClientFromEnv_explicitOptionOverridesEnv proves an explicit
// WithTimeout passed as an opt (what bogoslav-cli's --timeout does when
// the flag is actually given) wins over BOGOSLAV_TIMEOUT, the same way
// explicit code should win over an environment default.
func TestNewClientFromEnv_explicitOptionOverridesEnv(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_URL", "")
	t.Setenv("BOGOSLAV_TIMEOUT", "45s")

	c, err := NewClientFromEnv(WithTimeout(10 * time.Second))
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 10s (explicit option must win over BOGOSLAV_TIMEOUT)", c.httpClient.Timeout)
	}
}

// TestValidateTimeout_rejectsNegative pins ValidateTimeout's contract
// directly, since bogoslav-cli's --timeout flag validation reuses it.
func TestValidateTimeout_rejectsNegative(t *testing.T) {
	if err := ValidateTimeout(-1); err == nil {
		t.Error("ValidateTimeout(-1) error = nil, want an error")
	}
	if err := ValidateTimeout(0); err != nil {
		t.Errorf("ValidateTimeout(0) error = %v, want nil", err)
	}
	if err := ValidateTimeout(time.Second); err != nil {
		t.Errorf("ValidateTimeout(1s) error = %v, want nil", err)
	}
}

// TestParseTimeout_tableDriven pins ParseTimeout's contract: valid Go
// duration strings (including the bare "0"), rejecting negatives and
// garbage.
func TestParseTimeout_tableDriven(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "0s", want: 0},
		{in: "30s", want: 30 * time.Second},
		{in: "2m", want: 2 * time.Minute},
		{in: "-5s", wantErr: true},
		{in: "not-a-duration", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseTimeout(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTimeout(%q) error = nil, want an error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTimeout(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseTimeout(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
