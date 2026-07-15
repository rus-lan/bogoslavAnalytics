package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/cache"
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

func TestResolveUserCached_secondCallWithinTTLMakesZeroResolveCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		return 99, nil
	}}

	id1, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, now)
	if err != nil {
		t.Fatalf("ResolveUserCached() first call error = %v", err)
	}
	if id1 != 99 {
		t.Fatalf("ResolveUserCached() first call = %d, want 99", id1)
	}
	if resolver.calls != 1 {
		t.Fatalf("ResolveUserCached() first call made %d ResolveUserID calls, want 1", resolver.calls)
	}

	id2, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ResolveUserCached() second call error = %v", err)
	}
	if id2 != 99 {
		t.Fatalf("ResolveUserCached() second call = %d, want 99", id2)
	}
	if resolver.calls != 1 {
		t.Errorf("ResolveUserCached() second call within TTL made %d total ResolveUserID calls, want still 1 (cached)", resolver.calls)
	}
}

func TestResolveUserCached_expiredEntryMakesOneMoreCall(t *testing.T) {
	dir := t.TempDir()
	writtenAt := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	opts := cache.Options{Dir: dir, TTL: time.Hour}

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		return 99, nil
	}}

	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, writtenAt); err != nil {
		t.Fatalf("ResolveUserCached() first call error = %v", err)
	}

	later := writtenAt.Add(2 * time.Hour)
	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, later); err != nil {
		t.Fatalf("ResolveUserCached() expired call error = %v", err)
	}
	if resolver.calls != 2 {
		t.Errorf("ResolveUserCached() after TTL expiry made %d total ResolveUserID calls, want 2", resolver.calls)
	}
}

func TestResolveUserCached_refreshForcesOneMoreCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		return 99, nil
	}}

	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}
	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, now); err != nil {
		t.Fatalf("ResolveUserCached() first call error = %v", err)
	}

	refreshOpts := cache.Options{Dir: dir, TTL: cache.DefaultTTL, Refresh: true}
	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", refreshOpts, now); err != nil {
		t.Fatalf("ResolveUserCached() refresh call error = %v", err)
	}
	if resolver.calls != 2 {
		t.Errorf("ResolveUserCached() with Refresh=true made %d total ResolveUserID calls, want 2", resolver.calls)
	}
}

func TestResolveUserCached_numericUserAlwaysMakesZeroCalls(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		t.Fatalf("ResolveUserID called for numeric user %q", username)
		return 0, nil
	}}

	for i := range 3 {
		id, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "42", opts, now)
		if err != nil {
			t.Fatalf("ResolveUserCached() call %d error = %v", i, err)
		}
		if id != 42 {
			t.Errorf("ResolveUserCached() call %d = %d, want 42", i, id)
		}
	}
	if resolver.calls != 0 {
		t.Errorf("ResolveUserCached() with numeric user made %d ResolveUserID calls, want 0", resolver.calls)
	}
}

// TestResolveUserCached_keyIncludesGitlabURL is the hazard the doc
// comment names first: the same username on two different GitLab
// instances is two different people, so a resolution on one instance
// must never answer for the other.
func TestResolveUserCached_keyIncludesGitlabURL(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		return 99, nil
	}}

	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab-a.example.com", "alice", opts, now); err != nil {
		t.Fatalf("ResolveUserCached() first instance error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("ResolveUserCached() first instance made %d calls, want 1", resolver.calls)
	}

	if _, err := ResolveUserCached(context.Background(), resolver, "https://gitlab-b.example.com", "alice", opts, now); err != nil {
		t.Fatalf("ResolveUserCached() second instance error = %v", err)
	}
	if resolver.calls != 2 {
		t.Errorf("ResolveUserCached() same username on a different gitlab_url made %d total calls, want 2 (independent resolution)", resolver.calls)
	}
}

func TestResolveUserCached_malformedEntryIsCleanExtraCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}

	hash, err := cache.Hash(map[string]any{"gitlab_url": "https://gitlab.example.com", "username": "alice"})
	if err != nil {
		t.Fatalf("cache.Hash() error = %v", err)
	}
	path := filepath.Join(dir, cache.FileName(resolvedUserCacheName, hash, cache.ExtValue))
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write fixture file %s: %v", path, err)
	}

	resolver := &fakeUserResolver{fn: func(ctx context.Context, username string) (int64, error) {
		return 99, nil
	}}

	id, err := ResolveUserCached(context.Background(), resolver, "https://gitlab.example.com", "alice", opts, now)
	if err != nil {
		t.Fatalf("ResolveUserCached() error = %v", err)
	}
	if id != 99 {
		t.Errorf("ResolveUserCached() = %d, want 99 (falls through to a real resolve on a malformed entry)", id)
	}
	if resolver.calls != 1 {
		t.Errorf("ResolveUserCached() made %d ResolveUserID calls for a malformed entry, want 1 (clean extra call, not an error)", resolver.calls)
	}
}
