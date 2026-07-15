package search

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// mrSummary builds a gitlab.MergeRequestSummary with just the fields the
// window predicate and the (project_id, iid) key care about.
func mrSummary(projectID, iid int64, createdAt, updatedAt domain.Date) gitlab.MergeRequestSummary {
	return gitlab.MergeRequestSummary{
		MergeRequest: domain.MergeRequest{
			ProjectID: projectID,
			IID:       iid,
			CreatedAt: createdAt.Start(),
			UpdatedAt: updatedAt.Start(),
		},
	}
}

// filterByWindow mimics GitLab's real created_before/updated_after
// predicate over a fixed fixture, the same way a real merge request list
// endpoint would filter server-side. Used as both the "unsplit" baseline
// and the sub-window fake list function.
func filterByWindow(items []gitlab.MergeRequestSummary, w gitlab.MergeRequestWindow) []gitlab.MergeRequestSummary {
	var out []gitlab.MergeRequestSummary
	for _, it := range items {
		if it.CreatedAt.After(w.CreatedBefore.End()) {
			continue
		}
		if it.UpdatedAt.Before(w.UpdatedAfter.Start()) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func TestFetchMergeRequests_subWindowUnionExactness(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	window := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}

	fixture := []gitlab.MergeRequestSummary{
		mrSummary(1, 1, addDays(from, 1), addDays(from, 1)), // early only
		mrSummary(1, 2, addDays(to, -1), addDays(to, -1)),   // late only
		mrSummary(1, 3, addDays(from, 2), addDays(to, -2)),  // created before mid, updated after mid: overlaps both sub-windows
		mrSummary(1, 4, from, from),
		mrSummary(1, 5, to, to),
	}

	// Unsplit baseline: the same window, filtered in one pass, with no
	// bisection involved at all.
	want := filterByWindow(fixture, window)
	if len(want) != len(fixture) {
		t.Fatalf("test fixture setup: unsplit baseline = %d items, want all %d fixture items", len(want), len(fixture))
	}

	list := func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
		if w == window {
			// Force exactly one level of bisection at the top.
			return nil, gitlab.ErrPageLimitReached
		}
		return filterByWindow(fixture, w), nil
	}

	got, err := fetchMergeRequests(context.Background(), list, window)
	if err != nil {
		t.Fatalf("fetchMergeRequests() error = %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("fetchMergeRequests() returned %d items, want %d (same as the unsplit query) -- got=%+v", len(got), len(want), got)
	}

	counts := make(map[mrKey]int)
	for _, it := range got {
		counts[mrKey{projectID: it.ProjectID, iid: it.IID}]++
	}
	for _, it := range want {
		k := mrKey{projectID: it.ProjectID, iid: it.IID}
		if counts[k] != 1 {
			t.Errorf("merge request project=%d iid=%d appears %d times in the bisected union, want exactly 1", it.ProjectID, it.IID, counts[k])
		}
	}
	if len(counts) != len(want) {
		t.Errorf("bisected union has %d distinct merge requests, want %d", len(counts), len(want))
	}
}

func TestFetchMergeRequests_mrCreatedBeforeSplitAndUpdatedAfterAppearsExactlyOnce(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	window := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}
	mid := midDate(from, to)

	// The one merge request in this fixture straddles the split point on
	// purpose: created well before mid, updated well after mid.
	fixture := []gitlab.MergeRequestSummary{
		mrSummary(1, 42, addDays(from, 1), addDays(to, -1)),
	}

	list := func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
		if w == window {
			return nil, gitlab.ErrPageLimitReached
		}
		return filterByWindow(fixture, w), nil
	}

	got, err := fetchMergeRequests(context.Background(), list, window)
	if err != nil {
		t.Fatalf("fetchMergeRequests() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("fetchMergeRequests() returned %d items, want exactly 1 -- got=%+v (mid=%s)", len(got), got, mid)
	}
	if got[0].ProjectID != 1 || got[0].IID != 42 {
		t.Errorf("fetchMergeRequests()[0] = %+v, want project=1 iid=42", got[0])
	}
}

func TestFetchMergeRequests_windowNotSplittableAtSingleDayReturnsWrappedError(t *testing.T) {
	day := domain.NewDate(2026, time.March, 15)
	window := gitlab.MergeRequestWindow{CreatedBefore: day, UpdatedAfter: day}

	list := func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
		return nil, gitlab.ErrPageLimitReached
	}

	_, err := fetchMergeRequests(context.Background(), list, window)
	if !errors.Is(err, ErrWindowNotSplittable) {
		t.Fatalf("fetchMergeRequests() error = %v, want ErrWindowNotSplittable", err)
	}
}

func TestFetchMergeRequests_propagatesNonPageLimitError(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	window := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}
	wantErr := gitlab.ErrRateLimited

	list := func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
		return nil, wantErr
	}
	if _, err := fetchMergeRequests(context.Background(), list, window); !errors.Is(err, wantErr) {
		t.Fatalf("fetchMergeRequests() error = %v, want wrapping %v", err, wantErr)
	}
}

func TestFetchEvents_bisectsCleanlyOnPageLimitWithNoDuplicatesOrGaps(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	r := mustDateRange(from, to)

	fixture := []gitlab.CommentEvent{
		commentEvent(1, 1, false, addDays(from, 1).Start().Add(time.Hour)),
		commentEvent(1, 2, false, addDays(to, -1).Start().Add(time.Hour)),
		commentEvent(1, 3, false, midDate(from, to).Start().Add(time.Hour)),
	}
	want := len(fixture)

	list := func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
		if window == r {
			return nil, gitlab.ErrPageLimitReached
		}
		var out []gitlab.CommentEvent
		for _, e := range fixture {
			if window.Contains(e.CreatedAt) {
				out = append(out, e)
			}
		}
		return out, nil
	}

	got, err := fetchEvents(context.Background(), &fakeClient{commentEventsFn: list}, 42, r)
	if err != nil {
		t.Fatalf("fetchEvents() error = %v", err)
	}
	if len(got) != want {
		t.Fatalf("fetchEvents() returned %d events, want %d (no duplicates, no gaps) -- got=%+v", len(got), want, got)
	}
}

func TestFetchEvents_windowNotSplittableAtSingleDayReturnsWrappedError(t *testing.T) {
	day := domain.NewDate(2026, time.March, 15)
	r := mustDateRange(day, day)

	list := func(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error) {
		return nil, gitlab.ErrPageLimitReached
	}
	_, err := fetchEvents(context.Background(), &fakeClient{commentEventsFn: list}, 42, r)
	if !errors.Is(err, ErrWindowNotSplittable) {
		t.Fatalf("fetchEvents() error = %v, want ErrWindowNotSplittable", err)
	}
}

func TestSplitMergeRequestWindow_overlapsAroundMid(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	w := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}

	a, b, ok := splitMergeRequestWindow(w)
	if !ok {
		t.Fatal("splitMergeRequestWindow() ok = false, want true")
	}
	mid := midDate(from, to)
	if a.CreatedBefore != mid || a.UpdatedAfter != from {
		t.Errorf("window a = %+v, want CreatedBefore=%s UpdatedAfter=%s", a, mid, from)
	}
	if b.CreatedBefore != to || b.UpdatedAfter != mid {
		t.Errorf("window b = %+v, want CreatedBefore=%s UpdatedAfter=%s", b, to, mid)
	}
}

func TestSplitMergeRequestWindow_notSplittableWhenSingleDay(t *testing.T) {
	day := domain.NewDate(2026, time.March, 15)
	w := gitlab.MergeRequestWindow{CreatedBefore: day, UpdatedAfter: day}
	if _, _, ok := splitMergeRequestWindow(w); ok {
		t.Error("splitMergeRequestWindow() ok = true for a single-day window, want false")
	}
}

// TestSplitMergeRequestWindow_notSplittableWhenSpanIsExactlyOneDay guards the
// fixed point between span=0 (covered above) and span>=2 (covered below):
// when CreatedBefore is exactly one day after UpdatedAfter, midDate has no
// strictly-between day to land on and collapses to UpdatedAfter, which used
// to make sub-window b identical to the input window -- an infinite loop
// for fetchMergeRequests, since the same window would keep failing the same
// way forever.
func TestSplitMergeRequestWindow_notSplittableWhenSpanIsExactlyOneDay(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := addDays(from, 1)
	w := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}
	if _, _, ok := splitMergeRequestWindow(w); ok {
		t.Error("splitMergeRequestWindow() ok = true for a one-day-span window, want false")
	}
}

// TestSplitMergeRequestWindow_halvesAreStrictlySmallerThanInput sweeps
// several spans to make sure no other fixed point can hide the way the
// span=1 one did: for every span of two days or more, both returned
// sub-windows must be strictly smaller than the original window.
func TestSplitMergeRequestWindow_halvesAreStrictlySmallerThanInput(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	inputSpan := func(w gitlab.MergeRequestWindow) int {
		return int(w.CreatedBefore.Start().Sub(w.UpdatedAfter.Start()).Hours() / 24)
	}

	for _, spanDays := range []int{2, 3, 7, 30} {
		to := addDays(from, spanDays)
		w := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}

		a, b, ok := splitMergeRequestWindow(w)
		if !ok {
			t.Fatalf("span=%d: splitMergeRequestWindow() ok = false, want true", spanDays)
		}
		if aSpan := inputSpan(a); aSpan >= spanDays {
			t.Errorf("span=%d: sub-window a spans %d days, want strictly less than %d (a=%+v)", spanDays, aSpan, spanDays, a)
		}
		if bSpan := inputSpan(b); bSpan >= spanDays {
			t.Errorf("span=%d: sub-window b spans %d days, want strictly less than %d (b=%+v)", spanDays, bSpan, spanDays, b)
		}
		if b == w {
			t.Errorf("span=%d: sub-window b is identical to the input window %+v, splitting would never make progress", spanDays, w)
		}
	}
}

func TestSplitDateRange_isNonOverlapping(t *testing.T) {
	from := domain.NewDate(2026, time.March, 1)
	to := domain.NewDate(2026, time.March, 31)
	left, right, ok := splitDateRange(mustDateRange(from, to))
	if !ok {
		t.Fatal("splitDateRange() ok = false, want true")
	}
	if left.To.Before(left.From) || right.To.Before(right.From) {
		t.Fatalf("splitDateRange() produced an invalid half: left=%+v right=%+v", left, right)
	}
	if !left.To.Before(right.From) {
		t.Errorf("splitDateRange() halves overlap: left.To=%s right.From=%s, want left.To before right.From", left.To, right.From)
	}
	if left.From != from || right.To != to {
		t.Errorf("splitDateRange() does not cover the original range: left.From=%s (want %s) right.To=%s (want %s)", left.From, from, right.To, to)
	}
}

func TestSplitDateRange_notSplittableWhenSingleDay(t *testing.T) {
	day := domain.NewDate(2026, time.March, 15)
	if _, _, ok := splitDateRange(mustDateRange(day, day)); ok {
		t.Error("splitDateRange() ok = true for a single-day range, want false")
	}
}

// TestFetchMergeRequests_pageLimitAlwaysSpanOneTerminatesWithoutSpinning
// guards the fetchMergeRequests level of the same fixed point covered by
// TestSplitMergeRequestWindow_notSplittableWhenSpanIsExactlyOneDay: a
// span=1 window that keeps hitting the page limit must return a wrapped
// ErrWindowNotSplittable, not recurse on an identical sub-window forever.
//
// A real regression here would not just hang: unbounded recursion in Go
// blows the goroutine stack and crashes the whole process with a fatal,
// unrecoverable error, which would take the rest of this test binary down
// with it. So the actual call runs in a subprocess (the standard Go idiom
// for exercising code that might crash or hang the test binary), and this
// test only waits on it with a bound: a clean exit within the bound proves
// the fix, a non-zero exit surfaces a subprocess crash, and hitting the
// bound itself is treated as a failure rather than left to hang the suite.
//
// The helper's fake list only fails for the exact original window, not for
// every window unconditionally: a fake that fails unconditionally would
// let the recursion's first (single-day) sub-call fail on the pre-existing
// span=0 guard and return before ever reaching the second sub-call, which
// masks the bug instead of reproducing it. Failing only on the original
// window mirrors what a buggy split actually does -- collapse the second
// sub-window back to the same request -- while a genuinely smaller
// sub-window succeeds, the same way GitLab's real API would once the
// window is narrow enough to drop under the page limit.
func TestFetchMergeRequests_pageLimitAlwaysSpanOneTerminatesWithoutSpinning(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess_FetchMergeRequestsSpanOnePageLimited$", "-test.v=true")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper subprocess: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	const bound = 10 * time.Second
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("regression: fetchMergeRequests did not terminate cleanly for a page-limited span=1 window -- subprocess error=%v output=%s", err, out.String())
		}
		if !strings.Contains(out.String(), "PASS: ErrWindowNotSplittable") {
			t.Fatalf("helper subprocess exited cleanly but did not report the expected termination -- output=%s", out.String())
		}
	case <-time.After(bound):
		_ = cmd.Process.Kill()
		t.Fatalf("regression: fetchMergeRequests did not terminate within %s for a page-limited span=1 window -- suspected unbounded recursion (subprocess output so far=%s)", bound, out.String())
	}
}

// TestHelperProcess_FetchMergeRequestsSpanOnePageLimited does the actual
// work for TestFetchMergeRequests_pageLimitAlwaysSpanOneTerminatesWithoutSpinning.
// It only runs its real body when invoked as that test's subprocess; a plain
// `go test` run hits the env var guard and skips immediately, so the risky
// call never executes directly inside the main test process.
func TestHelperProcess_FetchMergeRequestsSpanOnePageLimited(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("only runs as the subprocess of TestFetchMergeRequests_pageLimitAlwaysSpanOneTerminatesWithoutSpinning")
	}

	from := domain.NewDate(2026, time.March, 1)
	to := addDays(from, 1)
	window := gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: from}

	list := func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
		if w == window {
			return nil, gitlab.ErrPageLimitReached
		}
		return nil, nil
	}

	_, err := fetchMergeRequests(context.Background(), list, window)
	if !errors.Is(err, ErrWindowNotSplittable) {
		fmt.Printf("FAIL: fetchMergeRequests() error = %v, want ErrWindowNotSplittable\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: ErrWindowNotSplittable")
	os.Exit(0)
}
