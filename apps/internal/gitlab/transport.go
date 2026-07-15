package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// request sends one GET request to path (relative to the API root) with
// query, retrying on 429 and 408 per TZ.md sections 5.2.6 and 6, and
// proactively slowing down beforehand per the most recently observed
// rate-limit headers (TZ.md section 6.2). On success the caller owns the
// returned response and must close its body.
func (c *Client) request(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if wait := c.limiter.nextDelay(); wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
		}

		req, err := c.newRequest(ctx, method, path, query)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gitlab: %s %s: %w", method, path, err)
		}
		c.limiter.observe(resp.Header)

		retry, retryErr, wait := retryDecision(resp.StatusCode, resp.Header, attempt, c.backoffBase, c.backoffCap)
		if !retry {
			return resp, nil
		}
		drainAndClose(resp)
		lastErr = retryErr

		if attempt == c.maxAttempts {
			break
		}
		if wait > 0 {
			if err := c.sleep(ctx, wait); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("gitlab: %s %s: exhausted %d attempts: %w", method, path, c.maxAttempts, lastErr)
}

// newRequest builds one GET request against the API root plus path,
// carrying the configured auth header.
func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+apiPrefix+path, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build request %s %s: %w", method, path, err)
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	req.Header.Set(c.authHeader, c.token)
	return req, nil
}

// retryDecision reports whether a response should be retried, the
// sentinel error it corresponds to, and how long to wait before retrying.
// It is a pure function so the retry policy can be unit tested without a
// real HTTP round trip.
func retryDecision(status int, header http.Header, attempt int, backoffBase, backoffCap time.Duration) (retry bool, err error, wait time.Duration) {
	switch status {
	case http.StatusTooManyRequests:
		if d, ok := parseRetryAfter(header); ok {
			return true, ErrRateLimited, d
		}
		return true, ErrRateLimited, backoffDelay(attempt, backoffBase, backoffCap)
	case http.StatusRequestTimeout:
		return true, ErrRequestTimeout, backoffDelay(attempt, backoffBase, backoffCap)
	default:
		return false, nil, 0
	}
}

// drainAndClose discards a response body before closing it, so the
// underlying connection can be reused for the retry.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// unexpectedStatus builds an error for a response whose status code has
// no documented meaning for the caller's endpoint.
func unexpectedStatus(resp *http.Response) error {
	return fmt.Errorf("unexpected status %d %s", resp.StatusCode, resp.Status)
}
