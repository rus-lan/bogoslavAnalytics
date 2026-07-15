package gitlab

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewClientFromEnv_missingToken(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	t.Setenv("GITLAB_TOKEN", "")

	_, err := NewClientFromEnv()
	if err == nil {
		t.Fatal("NewClientFromEnv() error = nil, want ErrMissingToken")
	}
	if !errors.Is(err, ErrMissingToken) {
		t.Errorf("NewClientFromEnv() error = %v, want ErrMissingToken", err)
	}
}

func TestNewClientFromEnv_defaultsBaseURL(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	t.Setenv("GITLAB_TOKEN", "secret")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func TestNewClientFromEnv_readsCustomURL(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://gitlab.example.com/")
	t.Setenv("GITLAB_TOKEN", "secret")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if c.baseURL != "https://gitlab.example.com" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

func TestClient_request_sendsConfiguredAuthHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "my-token")
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if gotHeader != "my-token" {
		t.Errorf("PRIVATE-TOKEN header = %q, want %q", gotHeader, "my-token")
	}
}

// TestClient_request_urlEmbeddedCredentialsStillAuthenticateTheRealRequest
// is the "(a) must keep working" half of the A2 fix: GITLAB_URL
// credentials (GitLab's own "https://oauth2:token@host" idiom) are
// left exactly as-is on Client.baseURL, so the actual outgoing request
// still authenticates via net/http's automatic Authorization: Basic
// derived from the URL's userinfo -- only the copy stamped into
// artifacts and cache keys (internal/app) is sanitized, never
// this one.
func TestClient_request_urlEmbeddedCredentialsStillAuthenticateTheRealRequest(t *testing.T) {
	var gotAuth, gotPrivateToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPrivateToken = r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", srv.URL, err)
	}
	base.User = url.UserPassword("gluser", "URLCANARY-99999")

	c := NewClient(base.String(), "my-token")
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("gluser:URLCANARY-99999"))
	if gotAuth != wantAuth {
		t.Errorf("Authorization header = %q, want %q (URL userinfo must still authenticate the real request)", gotAuth, wantAuth)
	}
	if gotPrivateToken != "my-token" {
		t.Errorf("PRIVATE-TOKEN header = %q, want %q", gotPrivateToken, "my-token")
	}
}

func TestClient_request_honorsCustomAuthHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "Bearer my-token", WithAuthHeader("Authorization"))
	resp, err := c.request(t.Context(), http.MethodGet, "/users", nil)
	if err != nil {
		t.Fatalf("request() error = %v", err)
	}
	resp.Body.Close()

	if gotHeader != "Bearer my-token" {
		t.Errorf("Authorization header = %q, want %q", gotHeader, "Bearer my-token")
	}
}
