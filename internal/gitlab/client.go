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

	// DefaultTimeout is the per-request deadline NewClientFromEnv applies
	// when BOGOSLAV_TIMEOUT is not set. It bounds one outgoing HTTP round
	// trip -- one page of a listing, one /discussions call -- not a whole
	// multi-call operation like a 28-page bruteforce walk: each page gets
	// its own fresh budget, since c.httpClient.Timeout (what WithTimeout
	// and this default ultimately set) is enforced fresh on every call to
	// httpClient.Do, and the retry loop in transport.go calls Do once per
	// attempt.
	//
	// 2 minutes is deliberately generous: a self-managed instance under
	// real load (large discussion threads, DB contention) can legitimately
	// take tens of seconds to answer one request, and this value must
	// outlast that without being mistaken for a hang. It is still finite,
	// which is the entire point of having a default at all -- a
	// genuinely dead connection (packets silently dropped by a firewall,
	// no RST) would otherwise sit on Linux's own TCP retransmission
	// timeout, which can run well past 15 minutes with no client-level
	// deadline set at all, exactly the "no Timeout means no client-level
	// deadline" gap BOGOSLAV_TIMEOUT/--timeout close.
	DefaultTimeout = 2 * time.Minute

	// timeoutEnvVar is BOGOSLAV_TIMEOUT (TZ.md section 2.5): a Go duration
	// string ("30s", "2m", ...), or "0" to disable the deadline entirely.
	// Read only by NewClientFromEnv, mirroring how GITLAB_URL and
	// GITLAB_TOKEN are read only there and not by NewClient itself.
	timeoutEnvVar = "BOGOSLAV_TIMEOUT"
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

	// timeout is nil until WithTimeout is called, so NewClient only ever
	// touches httpClient.Timeout when a caller actually asked it to --
	// never overwriting a value a caller-supplied http.Client from
	// WithHTTPClient already had, the way checkRedirect is always wired
	// in regardless (that one protects a secret; this one is a UX/
	// reliability knob with no such reason to be forced). WithTimeout(0)
	// still sets a non-nil *time.Duration pointing at 0, so it correctly
	// disables the deadline instead of doing nothing.
	timeout *time.Duration

	// now and sleep are overridden by tests in this package to make
	// timing-dependent behavior (the smoke test window, retry waits)
	// deterministic and fast.
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error
}

// Option configures a Client built by NewClient.
type Option func(*Client)

// WithHTTPClient sets the underlying http.Client. The default is a
// plain *http.Client equivalent to http.DefaultClient (but a distinct
// instance -- see NewClient).
//
// NewClient always overwrites h.CheckRedirect after every option runs,
// with the redirect policy that keeps the auth header from leaking to
// a different host (checkRedirect in transport.go). This is
// deliberate: the auth header is this package's own secret, so this
// package -- not the caller -- owns the decision about when it is safe
// to forward across a redirect, regardless of what CheckRedirect (if
// any) h already had.
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

// WithTimeout sets the deadline for each individual outgoing HTTP
// request: one page of a listing, one /discussions call, one retry
// attempt -- not a whole multi-call app-level operation. It works by
// setting the underlying http.Client's own Timeout field, which net/http
// enforces fresh on every call to Client.Do and which covers connect,
// TLS handshake, sending the request, waiting for response headers, and
// reading the response body -- every stage a slow GitLab instance can
// stall on. Zero disables it (net/http's own documented meaning for
// http.Client.Timeout): the request then has no deadline of its own
// beyond whatever the caller's context.Context already carries, and a
// truly hung connection blocks until ctx is canceled or the process is
// killed.
//
// This is independent of ctx cancellation, not a replacement for it:
// whichever of the two -- this Timeout or ctx.Done() -- fires first wins,
// exactly as net/http already documents; passing WithTimeout never stops
// a caller's own context.WithCancel/WithDeadline (or Ctrl-C via
// signal.NotifyContext) from aborting a request early.
//
// Calling WithTimeout is what makes NewClient touch httpClient.Timeout
// at all: leaving it out keeps this package's original behavior of no
// client-level deadline (whatever the supplied or default *http.Client's
// Timeout already was, zero by default), even if a later WithHTTPClient
// option in the same NewClient call replaces the *http.Client entirely --
// WithTimeout is always applied last, after every option has run, the
// same way checkRedirect is (see NewClient).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = &d }
}

// NewClient builds a client for the given base URL and token. baseURL is
// used as-is (minus a trailing slash); callers needing the GITLAB_URL /
// GITLAB_TOKEN environment convention should use NewClientFromEnv.
func NewClient(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		authHeader: authHeaderName,
		// A plain &http.Client{} has the exact same zero-value
		// behavior as http.DefaultClient (which is defined as
		// &http.Client{} too), but is a distinct instance: unlike
		// http.DefaultClient, it is safe to set CheckRedirect on
		// below without mutating a process-wide client some
		// unrelated package elsewhere in the same binary might also
		// be using.
		httpClient:  &http.Client{},
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
	// Always wired in last, after WithHTTPClient (if any) has had its
	// say about which *http.Client to use: see checkRedirect's doc
	// comment (transport.go) for what this protects and why it is not
	// optional.
	c.httpClient.CheckRedirect = c.checkRedirect
	// Also wired in last, but only if WithTimeout was actually called
	// (see the field's own doc comment): unlike CheckRedirect, no
	// timeout is forced on a caller who never asked for one.
	if c.timeout != nil {
		c.httpClient.Timeout = *c.timeout
	}
	return c
}

// NewClientFromEnv builds a client from GITLAB_URL (default
// https://gitlab.com), GITLAB_TOKEN (required; scope read_api, not
// api -- this client only ever issues GETs; TZ.md section 2.5), and
// BOGOSLAV_TIMEOUT (default DefaultTimeout; "0" disables the per-request
// deadline entirely -- TZ.md section 2.5).
//
// The env-derived timeout is applied via WithTimeout before opts, so any
// WithTimeout the caller passes in opts (for example bogoslav-cli's
// --timeout, when the flag was explicitly given) overrides it, the same
// precedence GITLAB_URL/GITLAB_TOKEN would have if this package ever grew
// a WithBaseURL/WithToken option: explicit code wins over environment.
func NewClientFromEnv(opts ...Option) (*Client, error) {
	baseURL := os.Getenv("GITLAB_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("gitlab: new client from env: %w", ErrMissingToken)
	}

	timeout := DefaultTimeout
	if raw := os.Getenv(timeoutEnvVar); raw != "" {
		parsed, err := ParseTimeout(raw)
		if err != nil {
			return nil, fmt.Errorf("gitlab: new client from env: %s=%q: %w", timeoutEnvVar, raw, err)
		}
		timeout = parsed
	}

	allOpts := make([]Option, 0, len(opts)+1)
	allOpts = append(allOpts, WithTimeout(timeout))
	allOpts = append(allOpts, opts...)
	return NewClient(baseURL, token, allOpts...), nil
}

// ParseTimeout parses raw as the value of BOGOSLAV_TIMEOUT: any Go
// duration string time.ParseDuration accepts ("30s", "2m", "1h"), or the
// bare "0" (also valid input to time.ParseDuration, needing no unit) to
// disable the deadline. See ValidateTimeout for why a negative value is
// rejected.
func ParseTimeout(raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("not a valid duration: %w", err)
	}
	if err := ValidateTimeout(d); err != nil {
		return 0, err
	}
	return d, nil
}

// ValidateTimeout rejects a negative duration: net/http does not
// document what a negative http.Client.Timeout does, and nothing about
// "disable the deadline" or "a real deadline" is naturally expressed by
// a negative duration, so this package does not guess at one. Used by
// ParseTimeout (BOGOSLAV_TIMEOUT) and by bogoslav-cli's --timeout flag,
// so both entry points reject the same values the same way.
func ValidateTimeout(d time.Duration) error {
	if d < 0 {
		return fmt.Errorf("must not be negative, got %s", d)
	}
	return nil
}
