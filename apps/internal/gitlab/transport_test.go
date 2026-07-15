package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSleeper records every requested sleep duration instead of actually
// waiting, keeping timing-dependent tests fast and deterministic.
type fakeSleeper struct {
	waits []time.Duration
}

func (f *fakeSleeper) sleep(_ context.Context, d time.Duration) error {
	f.waits = append(f.waits, d)
	return nil
}

func TestClient_request_honorsRetryAfterExactly(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token")
	c.sleep = sleeper.sleep

	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("server saw %d attempts, want 2", attempts)
	}
	if len(sleeper.waits) == 0 {
		t.Fatal("no sleep was requested")
	}
	last := sleeper.waits[len(sleeper.waits)-1]
	if last != 5*time.Second {
		t.Errorf("wait before retry = %v, want exactly 5s (Retry-After)", last)
	}
}

func TestClient_request_backoffWhenRateLimitHeadersAbsent(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			// No Retry-After, no RateLimit-* headers at all.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token", WithMaxAttempts(3), WithBackoff(100*time.Millisecond, 10*time.Second))
	c.sleep = sleeper.sleep

	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("server saw %d attempts, want 3", attempts)
	}
	if len(sleeper.waits) < 2 {
		t.Fatalf("sleep calls = %d, want at least 2 backoff waits", len(sleeper.waits))
	}
	first, second := sleeper.waits[0], sleeper.waits[1]
	if first != 100*time.Millisecond {
		t.Errorf("first backoff wait = %v, want 100ms", first)
	}
	if second != 200*time.Millisecond {
		t.Errorf("second backoff wait = %v, want 200ms (doubled)", second)
	}
}

func TestClient_request_exhaustsRetriesOnPersistent429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token", WithMaxAttempts(2), WithBackoff(time.Millisecond, time.Millisecond))
	c.sleep = sleeper.sleep

	_, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err == nil {
		t.Fatal("request() error = nil, want ErrRateLimited")
	}
}

func TestClient_request_retries408WithBackoff(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token", WithBackoff(50*time.Millisecond, time.Second))
	c.sleep = sleeper.sleep

	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("server saw %d attempts, want 2", attempts)
	}
	if len(sleeper.waits) == 0 || sleeper.waits[len(sleeper.waits)-1] != 50*time.Millisecond {
		t.Errorf("wait before retry = %v, want 50ms backoff", sleeper.waits)
	}
}

func TestClient_request_slowsDownWhenRateLimitRemainingIsLow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("RateLimit-Limit", "1000")
		w.Header().Set("RateLimit-Remaining", "1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token")
	c.sleep = sleeper.sleep

	// First call: no prior state, so no proactive wait yet.
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() #1 error = %v", err)
	}
	resp.Body.Close()
	if len(sleeper.waits) != 0 {
		t.Fatalf("waits after first request = %v, want none", sleeper.waits)
	}

	// Second call: the low RateLimit-Remaining observed on the first
	// response must make the client slow down before sending this one,
	// without waiting for an actual 429.
	resp2, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() #2 error = %v", err)
	}
	resp2.Body.Close()
	if len(sleeper.waits) != 1 {
		t.Fatalf("waits after second request = %v, want exactly 1 proactive wait", sleeper.waits)
	}
	if sleeper.waits[0] <= 0 {
		t.Errorf("proactive wait before request #2 = %v, want > 0", sleeper.waits[0])
	}
}

func TestClient_request_noSlowdownWhenRateLimitRemainingIsHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("RateLimit-Limit", "1000")
		w.Header().Set("RateLimit-Remaining", "900")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	sleeper := &fakeSleeper{}
	c := NewClient(srv.URL, "token")
	c.sleep = sleeper.sleep

	for i := 0; i < 2; i++ {
		resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
		if err != nil {
			t.Fatalf("request() #%d error = %v", i+1, err)
		}
		resp.Body.Close()
	}

	for i, w := range sleeper.waits {
		if w != 0 {
			t.Errorf("wait[%d] = %v, want 0 while RateLimit-Remaining stays healthy", i, w)
		}
	}
}

func TestRetryDecision_tableDriven(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		retryAfter string
		wantRetry  bool
		wantErr    error
	}{
		{"429 with retry-after", http.StatusTooManyRequests, "3", true, ErrRateLimited},
		{"429 without retry-after", http.StatusTooManyRequests, "", true, ErrRateLimited},
		{"408 always retries", http.StatusRequestTimeout, "", true, ErrRequestTimeout},
		{"200 never retries", http.StatusOK, "", false, nil},
		{"500 never retries", http.StatusInternalServerError, "", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.retryAfter != "" {
				h.Set("Retry-After", tc.retryAfter)
			}
			retry, err, wait := retryDecision(tc.status, h, 1, 100*time.Millisecond, time.Second)
			if retry != tc.wantRetry {
				t.Errorf("retry = %v, want %v", retry, tc.wantRetry)
			}
			if err != tc.wantErr {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantRetry && wait <= 0 {
				t.Errorf("wait = %v, want > 0", wait)
			}
		})
	}
}
