package search

import (
	"context"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func TestSelectStrategy_rangeOlderThanRetentionSelectsBruteforceWithoutSmoke(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	// Older than 3 years before fixedNow (2023-07-15).
	from := domain.NewDate(2023, time.January, 1)
	to := domain.NewDate(2023, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }}

	// smokeTestFn left nil: fakeClient panics if SelectStrategy tries to
	// call it, proving the retention check alone decides without a smoke
	// test.
	client := &fakeClient{}

	strategy, smoke, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyBruteforce {
		t.Errorf("SelectStrategy() strategy = %q, want %q", strategy, domain.StrategyBruteforce)
	}
	if smoke != "" {
		t.Errorf("SelectStrategy() smoke = %q, want empty (smoke test never ran)", smoke)
	}
}

func TestSelectStrategy_rangeWithinRetentionDoesNotAloneForceBruteforce(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	// Exactly at the edge: from is within the last 3 years.
	from := domain.NewDate(2024, time.January, 1)
	to := domain.NewDate(2024, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokePassed, nil
		},
	}

	strategy, _, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyEvents {
		t.Errorf("SelectStrategy() strategy = %q, want %q (range within retention, smoke passed)", strategy, domain.StrategyEvents)
	}
}

func TestSelectStrategy_strictForcesBruteforceWithoutSmoke(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Strict: true, Now: func() time.Time { return fixedNow }}

	client := &fakeClient{}

	strategy, smoke, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyBruteforce {
		t.Errorf("SelectStrategy() strategy = %q, want %q", strategy, domain.StrategyBruteforce)
	}
	if smoke != "" {
		t.Errorf("SelectStrategy() smoke = %q, want empty (smoke test never ran)", smoke)
	}
}

func TestSelectStrategy_smokeFailedSelectsBruteforce(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokeFailed, nil
		},
	}

	strategy, smoke, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyBruteforce {
		t.Errorf("SelectStrategy() strategy = %q, want %q", strategy, domain.StrategyBruteforce)
	}
	if smoke != domain.SmokeFailed {
		t.Errorf("SelectStrategy() smoke = %q, want %q", smoke, domain.SmokeFailed)
	}
}

func TestSelectStrategy_smokeUnknownAlsoSelectsBruteforce(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }}

	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			return domain.SmokeUnknown, nil
		},
	}

	strategy, smoke, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyBruteforce {
		t.Errorf("SelectStrategy() strategy = %q, want %q (unknown is conservative, TZ revision 2)", strategy, domain.StrategyBruteforce)
	}
	if smoke != domain.SmokeUnknown {
		t.Errorf("SelectStrategy() smoke = %q, want %q", smoke, domain.SmokeUnknown)
	}
}

func TestSelectStrategy_smokePassedAndRecentRangeSelectsEvents(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }}

	var gotUserID int64
	client := &fakeClient{
		smokeTestFn: func(ctx context.Context, userID int64) (domain.SmokeResult, error) {
			gotUserID = userID
			return domain.SmokePassed, nil
		},
	}

	strategy, smoke, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyEvents {
		t.Errorf("SelectStrategy() strategy = %q, want %q", strategy, domain.StrategyEvents)
	}
	if smoke != domain.SmokePassed {
		t.Errorf("SelectStrategy() smoke = %q, want %q", smoke, domain.SmokePassed)
	}
	if gotUserID != 42 {
		t.Errorf("SmokeTest called with user %d, want 42", gotUserID)
	}
}

func TestSelectStrategy_customRetentionYearsIsHonored(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	// Only 1 year old: within the default 3-year retention, but outside a
	// configured 1-year retention.
	from := domain.NewDate(2025, time.January, 1)
	to := domain.NewDate(2025, time.June, 30)
	p := Params{UserID: 42, Range: mustDateRange(from, to), MoreThan: 0}
	opts := Options{Now: func() time.Time { return fixedNow }, RetentionYears: 1}

	client := &fakeClient{}

	strategy, _, err := SelectStrategy(context.Background(), client, p, opts)
	if err != nil {
		t.Fatalf("SelectStrategy() error = %v", err)
	}
	if strategy != domain.StrategyBruteforce {
		t.Errorf("SelectStrategy() strategy = %q, want %q (from is older than the configured 1-year retention)", strategy, domain.StrategyBruteforce)
	}
}
