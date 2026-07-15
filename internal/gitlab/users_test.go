package gitlab

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func TestResolveUserID_oneMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/users" {
			t.Errorf("path = %q, want /api/v4/users", r.URL.Path)
		}
		if got := r.URL.Query().Get("username"); got != "alice" {
			t.Errorf("username query param = %q, want alice", got)
		}
		w.Write([]byte(`[{"id": 42, "username": "alice"}]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	id, err := c.ResolveUserID(t.Context(), "alice")
	if err != nil {
		t.Fatalf("ResolveUserID() error = %v", err)
	}
	if id != 42 {
		t.Errorf("ResolveUserID() = %d, want 42", id)
	}
}

func TestResolveUserID_emptyArrayIsUserNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	_, err := c.ResolveUserID(t.Context(), "ghost")
	if err == nil {
		t.Fatal("ResolveUserID() error = nil, want domain.ErrUserNotFound")
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("ResolveUserID() error = %v, want domain.ErrUserNotFound", err)
	}
}

func TestResolveUserID_caseInsensitive(t *testing.T) {
	// Simulate GitLab's own case-insensitive username matching: the fake
	// server matches regardless of case, and the client must pass the
	// value through unchanged rather than normalizing it itself.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("username")
		if strings.EqualFold(got, "AliceSmith") {
			w.Write([]byte(`[{"id": 99, "username": "alicesmith"}]`))
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")

	for _, username := range []string{"AliceSmith", "alicesmith", "ALICESMITH"} {
		id, err := c.ResolveUserID(t.Context(), username)
		if err != nil {
			t.Fatalf("ResolveUserID(%q) error = %v", username, err)
		}
		if id != 99 {
			t.Errorf("ResolveUserID(%q) = %d, want 99", username, id)
		}
	}
}

func TestResolveUserID_unexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	_, err := c.ResolveUserID(t.Context(), "alice")
	if err == nil {
		t.Fatal("ResolveUserID() error = nil, want an error for a 500 response")
	}
}
