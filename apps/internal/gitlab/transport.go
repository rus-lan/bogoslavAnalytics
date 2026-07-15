package gitlab

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

// maxRedirects caps how many redirects one logical request follows. It
// mirrors net/http's own built-in default (net/http's unexported
// defaultCheckRedirect also stops at 10): installing any custom
// CheckRedirect replaces that default entirely, so the cap has to be
// reimplemented here to keep the same ceiling instead of following
// redirects forever.
const maxRedirects = 10

// checkRedirect is installed as c.httpClient.CheckRedirect by
// NewClient. GitLab documents a 301 for a moved or renamed project
// (a legitimate, same-host redirect this method lets through
// unchanged), but nothing about a redirect guarantees the target is
// still GitLab: a hostile or compromised self-hosted instance, a
// typo'd GITLAB_URL, or a MITM injecting a 302 can point anywhere,
// including plain http://.
//
// net/http itself only strips a small closed list of header names
// (Authorization, Cookie, Proxy-Authorization, ...) on a cross-host
// redirect; that list is fixed inside the stdlib and has never heard
// of c.authHeader (PRIVATE-TOKEN by default). In the stdlib's own
// words, every other header is simply "copied" -- including to a
// different host. This method is what actually protects the token for
// a header name the stdlib doesn't special-case.
//
// The rule: the auth header survives a redirect only if the target is
// both (a) the exact same host as the very first request in the chain
// (via[0], compared case-insensitively on host:port, not the
// same-domain-or-subdomain relaxation net/http applies to cookies --
// there is no legitimate reason this Client's token needs to reach a
// second host at all) and (b) not a scheme downgrade from https to
// http on that host (an https request redirected to http on the same
// host would otherwise put the token on the wire in clear text, even
// though the original request was encrypted). Comparing against
// via[0] rather than the immediately preceding hop keeps the rule
// simple to reason about: a chain that leaves the original host and
// later returns to it exactly is treated the same as if that hop had
// been reached directly.
//
// Any request that fails both checks has the header deleted from
// req.Header in place before returning: req.Header is exactly what
// net/http sends on the next hop, so this is sufficient to keep it off
// the wire for that request, without needing CheckRedirect to abort
// the redirect altogether.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("gitlab: stopped after %d redirects", maxRedirects)
	}

	first := via[0].URL
	sameHost := strings.EqualFold(first.Host, req.URL.Host)
	downgraded := first.Scheme == "https" && req.URL.Scheme != "https"
	if !sameHost || downgraded {
		req.Header.Del(c.authHeader)
	}
	return nil
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
