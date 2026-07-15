package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// apiPrefix is the fixed REST API root. TZ.md section 5.0 spells it
	// out in full for one endpoint ("GET /api/v4/users?username=..."); the
	// rest of TZ.md abbreviates it away for every other endpoint, since it
	// is the same API.
	apiPrefix = "/api/v4"

	// defaultBaseURL is used when GITLAB_URL is not set (TZ.md section 2.5).
	defaultBaseURL = "https://gitlab.com"

	// authHeaderName is the header GitLab's REST API expects a token in.
	// Confirmed as the recommended mechanism by
	// https://docs.gitlab.com/api/rest/authentication/: "Pass the token
	// using the PRIVATE-TOKEN header (recommended) or other methods."
	// (Authorization: Bearer <token> is also documented as valid for a
	// personal access token, but PRIVATE-TOKEN is GitLab's own
	// recommendation.) Kept overridable via WithAuthHeader for the one
	// deployment that needs the alternative.
	authHeaderName = "PRIVATE-TOKEN"

	defaultMaxAttempts = 3
	defaultBackoffBase = 500 * time.Millisecond
	defaultBackoffCap  = 30 * time.Second
)

// Client is a GitLab REST API v4 client. It owns HTTP transport, offset
// pagination, and response-driven rate limiting (TZ.md sections 2.5 and 6).
// A Client is safe for concurrent use.
type Client struct {
	baseURL     string
	token       string
	authHeader  string
	httpClient  *http.Client
	limiter     *limiter
	maxAttempts int
	backoffBase time.Duration
	backoffCap  time.Duration

	// now and sleep are overridden by tests in this package to make
	// timing-dependent behavior (the smoke test window, retry waits)
	// deterministic and fast.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// Option configures a Client built by NewClient.
type Option func(*Client)

// WithHTTPClient sets the underlying http.Client. The default is
// http.DefaultClient.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithAuthHeader overrides the header name the token is sent in. The
// default is "PRIVATE-TOKEN".
func WithAuthHeader(name string) Option {
	return func(c *Client) { c.authHeader = name }
}

// WithMaxAttempts overrides how many times a request is attempted in total
// (initial attempt plus retries) before a 429 or 408 gives up. The default
// is 3 (TZ.md section 6.8: "дефолт 3, настраивается").
func WithMaxAttempts(n int) Option {
	return func(c *Client) { c.maxAttempts = n }
}

// WithBackoff overrides the base delay and cap used by the exponential
// backoff fallback (TZ.md section 6.4).
func WithBackoff(base, cap time.Duration) Option {
	return func(c *Client) { c.backoffBase = base; c.backoffCap = cap }
}

// NewClient builds a client for the given base URL and token. baseURL is
// used as-is (minus a trailing slash); callers needing the GITLAB_URL /
// GITLAB_TOKEN environment convention should use NewClientFromEnv.
func NewClient(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		token:       token,
		authHeader:  authHeaderName,
		httpClient:  http.DefaultClient,
		limiter:     newLimiter(),
		maxAttempts: defaultMaxAttempts,
		backoffBase: defaultBackoffBase,
		backoffCap:  defaultBackoffCap,
		now:         time.Now,
		sleep:       sleepCtx,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewClientFromEnv builds a client from GITLAB_URL (default
// https://gitlab.com) and GITLAB_TOKEN (required; scope read_user or api,
// TZ.md section 2.5).
func NewClientFromEnv(opts ...Option) (*Client, error) {
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("gitlab: new client from env: %w", ErrMissingToken)
	}
	return NewClient(baseURL, token, opts...), nil
}
