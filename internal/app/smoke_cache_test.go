package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func TestCachingSmokeClient_secondCallWithinTTLIsCached(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}
	wrapped := &cachingSmokeClient{
		Client:    underlying,
		gitlabURL: "https://gitlab.example.com",
		opts:      cache.Options{Dir: dir, TTL: cache.DefaultTTL},
		now:       func() time.Time { return now },
	}

	result, err := wrapped.SmokeTest(context.Background(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() first call error = %v", err)
	}
	if result != domain.SmokePassed {
		t.Fatalf("SmokeTest() first call = %q, want %q", result, domain.SmokePassed)
	}
	if underlying.smokeTestCalls != 1 {
		t.Fatalf("SmokeTest() first call made %d calls to the underlying client, want 1", underlying.smokeTestCalls)
	}

	wrapped.now = func() time.Time { return now.Add(time.Minute) }
	result2, err := wrapped.SmokeTest(context.Background(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() second call error = %v", err)
	}
	if result2 != domain.SmokePassed {
		t.Errorf("SmokeTest() second call = %q, want %q (from cache)", result2, domain.SmokePassed)
	}
	if underlying.smokeTestCalls != 1 {
		t.Errorf("SmokeTest() second call within TTL made %d total calls to the underlying client, want still 1 (cached)", underlying.smokeTestCalls)
	}
}

func TestCachingSmokeClient_expiredEntryCallsAgain(t *testing.T) {
	dir := t.TempDir()
	writtenAt := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}
	wrapped := &cachingSmokeClient{
		Client:    underlying,
		gitlabURL: "https://gitlab.example.com",
		opts:      cache.Options{Dir: dir, TTL: time.Hour},
		now:       func() time.Time { return writtenAt },
	}

	if _, err := wrapped.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() first call error = %v", err)
	}

	wrapped.now = func() time.Time { return writtenAt.Add(2 * time.Hour) }
	if _, err := wrapped.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() expired call error = %v", err)
	}
	if underlying.smokeTestCalls != 2 {
		t.Errorf("SmokeTest() after TTL expiry made %d total calls, want 2", underlying.smokeTestCalls)
	}
}

func TestCachingSmokeClient_refreshForcesCallAgain(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}
	wrapped := &cachingSmokeClient{
		Client:    underlying,
		gitlabURL: "https://gitlab.example.com",
		opts:      cache.Options{Dir: dir, TTL: cache.DefaultTTL},
		now:       func() time.Time { return now },
	}

	if _, err := wrapped.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() first call error = %v", err)
	}

	wrapped.opts.Refresh = true
	if _, err := wrapped.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() refresh call error = %v", err)
	}
	if underlying.smokeTestCalls != 2 {
		t.Errorf("SmokeTest() with Refresh=true made %d total calls, want 2", underlying.smokeTestCalls)
	}
}

// TestCachingSmokeClient_keyIncludesUserID is the hazard the doc comment
// names: the probe needs a user with actual thread replies, so it can
// legitimately come back SmokeUnknown for one user and SmokePassed for
// another on the exact same instance -- keying on gitlab_url alone would
// let user 1's inconclusive result silently answer for user 2 too.
func TestCachingSmokeClient_keyIncludesUserID(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			if userID == 1 {
				return domain.SmokeUnknown, nil
			}
			return domain.SmokePassed, nil
		},
	}
	wrapped := &cachingSmokeClient{
		Client:    underlying,
		gitlabURL: "https://gitlab.example.com",
		opts:      cache.Options{Dir: dir, TTL: cache.DefaultTTL},
		now:       func() time.Time { return now },
	}

	resultA, err := wrapped.SmokeTest(context.Background(), 1)
	if err != nil {
		t.Fatalf("SmokeTest(user 1) error = %v", err)
	}
	if resultA != domain.SmokeUnknown {
		t.Fatalf("SmokeTest(user 1) = %q, want %q", resultA, domain.SmokeUnknown)
	}

	resultB, err := wrapped.SmokeTest(context.Background(), 2)
	if err != nil {
		t.Fatalf("SmokeTest(user 2) error = %v", err)
	}
	if resultB != domain.SmokePassed {
		t.Errorf("SmokeTest(user 2) = %q, want %q -- user 1's unknown result must not poison user 2's", resultB, domain.SmokePassed)
	}
	if underlying.smokeTestCalls != 2 {
		t.Errorf("SmokeTest() for two different users made %d total calls to the underlying client, want 2 (independent keys)", underlying.smokeTestCalls)
	}
}

func TestCachingSmokeClient_keyIncludesGitlabURL(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}
	opts := cache.Options{Dir: dir, TTL: cache.DefaultTTL}
	wrappedA := &cachingSmokeClient{Client: underlying, gitlabURL: "https://gitlab-a.example.com", opts: opts, now: func() time.Time { return now }}
	wrappedB := &cachingSmokeClient{Client: underlying, gitlabURL: "https://gitlab-b.example.com", opts: opts, now: func() time.Time { return now }}

	if _, err := wrappedA.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() instance A error = %v", err)
	}
	if _, err := wrappedB.SmokeTest(context.Background(), 42); err != nil {
		t.Fatalf("SmokeTest() instance B error = %v", err)
	}
	if underlying.smokeTestCalls != 2 {
		t.Errorf("SmokeTest() for the same user on two gitlab_urls made %d total calls, want 2 (independent keys)", underlying.smokeTestCalls)
	}
}

func TestCachingSmokeClient_malformedEntryIsCleanExtraCall(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	hash, err := cache.Hash(map[string]any{"gitlab_url": "https://gitlab.example.com", "user_id": int64(42)})
	if err != nil {
		t.Fatalf("cache.Hash() error = %v", err)
	}
	path := filepath.Join(dir, cache.FileName(smokeCacheName, hash, cache.ExtValue))
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write fixture file %s: %v", path, err)
	}

	underlying := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}
	wrapped := &cachingSmokeClient{
		Client:    underlying,
		gitlabURL: "https://gitlab.example.com",
		opts:      cache.Options{Dir: dir, TTL: cache.DefaultTTL},
		now:       func() time.Time { return now },
	}

	result, err := wrapped.SmokeTest(context.Background(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if result != domain.SmokePassed {
		t.Errorf("SmokeTest() = %q, want %q (falls through to a real probe on a malformed entry)", result, domain.SmokePassed)
	}
	if underlying.smokeTestCalls != 1 {
		t.Errorf("SmokeTest() made %d calls for a malformed entry, want 1 (clean extra call, not an error)", underlying.smokeTestCalls)
	}
}
