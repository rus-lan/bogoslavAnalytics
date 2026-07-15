package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestClient_request_crossHostRedirectDropsAuthHeader is the proof for
// the A1 fix: a redirect to a different host must never carry
// c.authHeader (PRIVATE-TOKEN by default) along with it. This asserts
// on what the SECOND host actually received, not on the first
// request's own headers, so it fails if checkRedirect is reverted (no
// CheckRedirect at all, or one that never deletes the header) --
// net/http's own closed list (Authorization, Cookie, ...) does not
// cover a custom header name, so PRIVATE-TOKEN would otherwise be
// copied to the attacker host unchanged.
func TestClient_request_crossHostRedirectDropsAuthHeader(t *testing.T) {
	var attackerHeader string
	var attackerHit bool
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerHit = true
		attackerHeader = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`[]`))
	}))
	defer attacker.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attacker.URL+"/api/v4/users", http.StatusFound)
	}))
	defer origin.Close()

	c := NewClient(origin.URL, "SECRET-CANARY-12345")
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if !attackerHit {
		t.Fatal("attacker host never received a request; test setup is broken")
	}
	if attackerHeader != "" {
		t.Errorf("attacker host received PRIVATE-TOKEN = %q, want empty (cross-host redirect must drop the auth header)", attackerHeader)
	}
}

// TestClient_request_sameHostRedirectKeepsAuthHeader is the moved
// -project case (TZ.md/GitLab documents a 301 when a project is
// renamed): a redirect that stays on the same host must keep working
// exactly as it did before CheckRedirect was introduced.
func TestClient_request_sameHostRedirectKeepsAuthHeader(t *testing.T) {
	var gotHeader string
	var redirected bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/users" {
			redirected = true
			http.Redirect(w, r, "/api/v4/moved-users", http.StatusMovedPermanently)
			return
		}
		gotHeader = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`[]`))
	}))
	defer origin.Close()

	c := NewClient(origin.URL, "my-token")
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if !redirected {
		t.Fatal("origin never issued the same-host redirect; test setup is broken")
	}
	if gotHeader != "my-token" {
		t.Errorf("same-host redirect target received PRIVATE-TOKEN = %q, want %q", gotHeader, "my-token")
	}
}

// TestCheckRedirect_httpsToHttpDowngradeOnSameHostDropsAuthHeader tests
// the documented downgrade rule directly against checkRedirect: an
// https request redirected to plain http on the exact same host must
// drop the auth header, since it would otherwise cross the network in
// clear text on the redirected request.
//
// This is a direct unit test of checkRedirect, not an httptest round
// trip: httptest.NewServer and httptest.NewTLSServer each bind their
// own listener on their own port, so there is no way to make one
// plain-HTTP host and one HTTPS host share the identical host:port
// httptest would need for a genuine same-host scheme downgrade -- any
// such attempt fails at the TLS/TCP layer before either handler ever
// runs, which would prove nothing about header handling. Calling
// checkRedirect directly with hand-built requests tests the exact rule
// documented on it without that dead end.
func TestCheckRedirect_httpsToHttpDowngradeOnSameHostDropsAuthHeader(t *testing.T) {
	c := NewClient("https://gitlab.example.com", "my-token")

	firstURL, err := url.Parse("https://gitlab.example.com/api/v4/users")
	if err != nil {
		t.Fatalf("url.Parse(first) error = %v", err)
	}
	redirectURL, err := url.Parse("http://gitlab.example.com/api/v4/users")
	if err != nil {
		t.Fatalf("url.Parse(redirect) error = %v", err)
	}

	via := []*http.Request{{URL: firstURL}}
	req := &http.Request{URL: redirectURL, Header: http.Header{"Private-Token": []string{"my-token"}}}

	if err := c.checkRedirect(req, via); err != nil {
		t.Fatalf("checkRedirect() error = %v, want nil (downgrade is handled by dropping the header, not by failing the redirect)", err)
	}
	if got := req.Header.Get("PRIVATE-TOKEN"); got != "" {
		t.Errorf("PRIVATE-TOKEN after https->http downgrade on the same host = %q, want empty", got)
	}
}

// TestCheckRedirect_httpsToHttpsSameHostKeepsAuthHeader is the control
// for the downgrade test above: no scheme change, same host, must keep
// the header.
func TestCheckRedirect_httpsToHttpsSameHostKeepsAuthHeader(t *testing.T) {
	c := NewClient("https://gitlab.example.com", "my-token")

	firstURL, _ := url.Parse("https://gitlab.example.com/api/v4/users")
	redirectURL, _ := url.Parse("https://gitlab.example.com/api/v4/moved-users")

	via := []*http.Request{{URL: firstURL}}
	req := &http.Request{URL: redirectURL, Header: http.Header{"Private-Token": []string{"my-token"}}}

	if err := c.checkRedirect(req, via); err != nil {
		t.Fatalf("checkRedirect() error = %v, want nil", err)
	}
	if got := req.Header.Get("PRIVATE-TOKEN"); got != "my-token" {
		t.Errorf("PRIVATE-TOKEN after same-host, same-scheme redirect = %q, want %q", got, "my-token")
	}
}

// TestClient_request_redirectLimitStopsFollowingForever proves the
// redirect cap decision: following redirects is wanted (GitLab's
// moved-project 301 must keep working, see the same-host test above),
// but not forever. checkRedirect mirrors net/http's own default cap of
// 10, so a server that redirects to itself indefinitely still makes
// the request fail instead of looping forever, exactly as it would
// have with CheckRedirect left nil.
func TestClient_request_redirectLimitStopsFollowingForever(t *testing.T) {
	var hits int
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, "/api/v4/users", http.StatusFound)
	}))
	defer origin.Close()

	c := NewClient(origin.URL, "my-token")
	_, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err == nil {
		t.Fatal("request() error = nil, want a stopped-after-N-redirects error")
	}

	// The initial request plus (maxRedirects-1) redirects actually reach
	// the server: checkRedirect is asked about the next hop before it
	// is sent, and refuses once via already holds maxRedirects entries
	// -- so the maxRedirects-th redirect is the one never sent. This
	// matches net/http's own default checkRedirect, which uses the
	// identical len(via) >= 10 check.
	if hits != maxRedirects {
		t.Errorf("origin saw %d hits, want %d (initial request + %d redirects actually sent)", hits, maxRedirects, maxRedirects-1)
	}
}
