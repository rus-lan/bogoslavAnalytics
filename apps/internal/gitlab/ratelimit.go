package gitlab

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// limiter turns the most recently observed RateLimit-Limit and
// RateLimit-Remaining response headers into a proactive delay before the
// next request. Per TZ.md section 6.1-6.4, the decision is made fresh from
// each response -- never pinned to a hardcoded rate constant (gitlab.com's
// 2000/min, self-managed's 7200/3600s) and never pinned to a specific
// endpoint path.
type limiter struct {
	mu         sync.Mutex
	hasHeaders bool
	limit      int
	remaining  int

	// slowdownFraction is the share of limit at or below which the
	// limiter starts adding delay before the next request. unit is the
	// delay added per unit that remaining sits below that threshold, so
	// the wait grows the closer remaining gets to zero.
	slowdownFraction float64
	unit             time.Duration
}

func newLimiter() *limiter {
	return &limiter{
		slowdownFraction: 0.1,
		unit:             50 * time.Millisecond,
	}
}

// observe records the rate-limit headers of a response, if present. A
// response with no headers (or headers that fail to parse) clears the
// prior state, since TZ.md requires the decision to be made per response,
// never carried over from an earlier, unrelated call.
func (l *limiter) observe(h http.Header) {
	limitStr := h.Get("RateLimit-Limit")
	remainingStr := h.Get("RateLimit-Remaining")
	limitVal, errLimit := strconv.Atoi(limitStr)
	remainingVal, errRemaining := strconv.Atoi(remainingStr)

	l.mu.Lock()
	defer l.mu.Unlock()
	if limitStr == "" || remainingStr == "" || errLimit != nil || errRemaining != nil {
		l.hasHeaders = false
		return
	}
	l.hasHeaders = true
	l.limit = limitVal
	l.remaining = remainingVal
}

// nextDelay returns how long to wait before sending the next request,
// given the most recently observed state. It is zero once no rate-limit
// headers have ever been observed, and stays zero while remaining sits
// comfortably above the slowdown threshold.
func (l *limiter) nextDelay() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.hasHeaders || l.limit <= 0 {
		return 0
	}
	threshold := int(float64(l.limit) * l.slowdownFraction)
	if threshold < 1 {
		threshold = 1
	}
	if l.remaining > threshold {
		return 0
	}
	// +1 so the wait is already positive right at the threshold, not just
	// once remaining drops below it.
	deficit := threshold - l.remaining + 1
	return time.Duration(deficit) * l.unit
}

// parseRetryAfter reads the Retry-After header of a 429 response: either
// an integer number of seconds or an HTTP-date, per RFC 9110. TZ.md
// section 6.3 requires this to be honored exactly, independent of whether
// any other rate-limit header is present.
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// backoffDelay computes an exponential backoff delay for the given attempt
// (1-based). It is the fallback TZ.md section 6.4 requires when a response
// carries no rate-limit headers to adapt to, and the retry strategy for
// 408 responses (TZ.md sections 5.2.6 and 6.8).
func backoffDelay(attempt int, base, cap time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := base
	for i := 1; i < attempt; i++ {
		if d > cap/2 {
			return cap
		}
		d *= 2
	}
	if d > cap {
		return cap
	}
	return d
}

// sleepCtx waits for d, honoring context cancellation.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
