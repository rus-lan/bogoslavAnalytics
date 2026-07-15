package gitlab

import (
	"net/http"
	"testing"
	"time"
)

func TestLimiter_nextDelay_slowsDownAsRemainingApproachesZero(t *testing.T) {
	cases := []struct {
		name      string
		limit     string
		remaining string
		wantZero  bool
	}{
		{"plenty remaining", "2000", "1900", true},
		{"comfortably above threshold", "2000", "300", true},
		{"remaining at threshold", "2000", "200", false},
		{"remaining approaching zero", "2000", "5", false},
		{"remaining exhausted", "2000", "0", false},
	}

	var previousDelay time.Duration = -1
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLimiter()
			h := http.Header{}
			h.Set("RateLimit-Limit", tc.limit)
			h.Set("RateLimit-Remaining", tc.remaining)
			l.observe(h)

			got := l.nextDelay()
			if tc.wantZero && got != 0 {
				t.Errorf("nextDelay() = %v, want 0", got)
			}
			if !tc.wantZero && got <= 0 {
				t.Errorf("nextDelay() = %v, want > 0", got)
			}
			_ = previousDelay
		})
	}
}

func TestLimiter_nextDelay_growsAsRemainingShrinks(t *testing.T) {
	l := newLimiter()
	h := http.Header{}
	h.Set("RateLimit-Limit", "1000")

	h.Set("RateLimit-Remaining", "50")
	l.observe(h)
	higher := l.nextDelay()

	h.Set("RateLimit-Remaining", "5")
	l.observe(h)
	lower := l.nextDelay()

	if lower <= higher {
		t.Errorf("nextDelay() at remaining=5 (%v) should exceed remaining=50 (%v)", lower, higher)
	}
}

func TestLimiter_nextDelay_zeroWithoutHeaders(t *testing.T) {
	l := newLimiter()
	if got := l.nextDelay(); got != 0 {
		t.Errorf("nextDelay() with no observation = %v, want 0", got)
	}

	h := http.Header{}
	l.observe(h)
	if got := l.nextDelay(); got != 0 {
		t.Errorf("nextDelay() after empty headers = %v, want 0", got)
	}
}

func TestLimiter_observe_clearsStateWhenHeadersMissingOnLaterResponse(t *testing.T) {
	l := newLimiter()

	low := http.Header{}
	low.Set("RateLimit-Limit", "1000")
	low.Set("RateLimit-Remaining", "1")
	l.observe(low)
	if got := l.nextDelay(); got <= 0 {
		t.Fatalf("nextDelay() after low remaining = %v, want > 0", got)
	}

	l.observe(http.Header{})
	if got := l.nextDelay(); got != 0 {
		t.Errorf("nextDelay() after a response without headers = %v, want 0 (decision is per response, not sticky)", got)
	}
}

func TestParseRetryAfter_integerSeconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "7")

	got, ok := parseRetryAfter(h)
	if !ok {
		t.Fatal("parseRetryAfter() ok = false, want true")
	}
	if got != 7*time.Second {
		t.Errorf("parseRetryAfter() = %v, want 7s", got)
	}
}

func TestParseRetryAfter_httpDate(t *testing.T) {
	target := time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)
	h := http.Header{}
	h.Set("Retry-After", target.Format(http.TimeFormat))

	got, ok := parseRetryAfter(h)
	if !ok {
		t.Fatal("parseRetryAfter() ok = false, want true")
	}
	// Allow a small margin: parseRetryAfter measures against time.Now()
	// internally, and the header itself only has one-second resolution.
	if got < 8*time.Second || got > 11*time.Second {
		t.Errorf("parseRetryAfter() = %v, want close to 10s", got)
	}
}

func TestParseRetryAfter_missing(t *testing.T) {
	_, ok := parseRetryAfter(http.Header{})
	if ok {
		t.Error("parseRetryAfter() ok = true, want false for a missing header")
	}
}

func TestBackoffDelay_doublesUntilCap(t *testing.T) {
	base := 100 * time.Millisecond
	cap := 1 * time.Second

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second},
		{6, 1 * time.Second},
	}
	for _, tc := range cases {
		if got := backoffDelay(tc.attempt, base, cap); got != tc.want {
			t.Errorf("backoffDelay(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}
