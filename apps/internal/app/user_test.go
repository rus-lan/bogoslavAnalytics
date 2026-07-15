package app

import (
	"context"
	"testing"
)

// fakeUserResolver implements UserResolver in-memory for ResolveUser
// tests.
type fakeUserResolver struct {
	fn    func(ctx context.Context, username string) (int64, error)
	calls int
}

func (f *fakeUserResolver) ResolveUserID(ctx context.Context, username string) (int64, error) {
	f.calls++
	return f.fn(ctx, username)
}

func TestResolveUser_numericUserMakesNoCalls(t *testing.T) {
	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		t.Fatalf("ResolveUserID called for numeric user %q", username)
		return 0, nil
	}}

	id, err := ResolveUser(context.Background(), resolver, "42")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if id != 42 {
		t.Errorf("ResolveUser() = %d, want 42", id)
	}
	if resolver.calls != 0 {
		t.Errorf("ResolveUser() called ResolveUserID %d times for a numeric user, want 0", resolver.calls)
	}
}

func TestResolveUser_usernameResolvesOnce(t *testing.T) {
	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		if username != "alice" {
			t.Fatalf("ResolveUserID called with %q, want alice", username)
		}
		return 99, nil
	}}

	id, err := ResolveUser(context.Background(), resolver, "alice")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}
	if id != 99 {
		t.Errorf("ResolveUser() = %d, want 99", id)
	}
	if resolver.calls != 1 {
		t.Errorf("ResolveUser() called ResolveUserID %d times, want exactly 1", resolver.calls)
	}
}
