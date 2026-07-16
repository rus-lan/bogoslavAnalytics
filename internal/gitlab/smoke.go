package gitlab

import (
	"context"
	"errors"
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

const (
	// smokeWindowDays is the recent window the smoke test samples (TZ.md
	// section 5.5.1: "дефолт: последние 90 дней").
	smokeWindowDays = 90
	// smokeMaxCandidates bounds how many distinct merge requests the smoke
	// test will fetch /discussions for (TZ.md section 5.5.2). It is a hard
	// call budget, not a target: every candidate inside the budget is
	// checked, and only a confirmed undercount stops the scan early --
	// the bound exists purely to keep worst-case cost fixed, not to cut
	// the scan short once things look fine so far (see the package doc
	// comment on SmokeTest for why stopping early on "looks fine" was the
	// actual defect being fixed here).
	smokeMaxCandidates = 20
)

// SmokeTest runs the events-API-undercount instance-capability detection
// described in TZ.md section 5.5: it checks whether this GitLab instance's
// Events API silently reports fewer comment events for a merge request
// than that merge request's /discussions actually holds for the same
// user and window. Only the detection lives here; deciding what strategy
// the result should select is the responsibility of search/'s
// autoselector (TZ.md section 5.3).
//
// Procedure:
//  1. Fetch userID's comment events over the last smokeWindowDays days,
//     without a target_type filter.
//  2. Pick up to smokeMaxCandidates merge requests referenced by those
//     events (newest-first, per mrCandidatesFromEvents) and fetch each
//     one's exact /discussions count over the same window.
//  3. A candidate is comparable if the user has at least one note (of any
//     type -- DiscussionNote, DiffNote, or a plain untyped comment) on it
//     inside the window; a candidate with zero such notes has nothing to
//     compare against and is skipped without affecting the verdict.
//  4. Every comparable candidate inside the budget is checked, not just
//     the first one found: if its raw event count is lower than its
//     exact /discussions count, the events API is losing notes for this
//     user on this instance: SmokeFailed, returned immediately. A single
//     confirmed undercount is sufficient evidence regardless of how many
//     other candidates looked fine -- this is what the real incident
//     this test encodes required (see smoke_test.go and TZ.md 5.5.5): a
//     user whose only affected merge requests are not the first ones
//     sampled must still get a real verdict, not a false "passed" from
//     having stopped early on clean candidates.
//  5. If no candidate in the whole budget was comparable, the result is
//     inconclusive: SmokeUnknown (the caller must treat this as
//     equivalent to a failure and fall back to bruteforce, per TZ.md
//     section 5.5.4 -- that fallback decision itself belongs to
//     search/). This is meant to be rare: it only happens when the user
//     genuinely has no recent comment activity the probe can use, not as
//     a side effect of which merge requests happened to be newest.
//  6. Otherwise: SmokePassed.
func (c *Client) SmokeTest(ctx context.Context, userID int64) (domain.SmokeResult, error) {
	now := c.now().UTC()
	to := domain.NewDate(now.Year(), now.Month(), now.Day())
	fromInstant := now.AddDate(0, 0, -smokeWindowDays)
	from := domain.NewDate(fromInstant.Year(), fromInstant.Month(), fromInstant.Day())

	window, err := domain.NewDateRange(from, to)
	if err != nil {
		return domain.SmokeUnknown, fmt.Errorf("gitlab: smoke test window: %w", err)
	}

	events, err := c.CommentEvents(ctx, userID, window)
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return domain.SmokeUnknown, fmt.Errorf("gitlab: smoke test events: %w", err)
	}

	candidates := mrCandidatesFromEvents(events, smokeMaxCandidates)
	if len(candidates) == 0 {
		return domain.SmokeUnknown, nil
	}

	comparable := false
	for _, cand := range candidates {
		discussions, err := c.Discussions(ctx, NumericID(cand.projectID), cand.mrIID)
		if err != nil && !errors.Is(err, ErrPageLimitReached) {
			return domain.SmokeUnknown, fmt.Errorf("gitlab: smoke test discussions: %w", err)
		}

		exact := countUserNotes(discussions, userID, window)
		if exact == 0 {
			continue
		}
		comparable = true
		if cand.eventCount < exact {
			return domain.SmokeFailed, nil
		}
	}
	if !comparable {
		return domain.SmokeUnknown, nil
	}
	return domain.SmokePassed, nil
}

// mrCandidate is one merge request sampled from recent comment events,
// carrying the raw event count observed for it.
type mrCandidate struct {
	projectID  int64
	mrIID      int64
	eventCount int
}

// mrCandidatesFromEvents maps events onto (project_id, note.noteable_iid)
// pairs, keeping only events for merge request notes that are not system
// notes -- the same candidate rule TZ.md section 5.1.3 uses for the events
// search strategy -- then returns up to max candidates in first-seen
// order.
func mrCandidatesFromEvents(events []CommentEvent, max int) []mrCandidate {
	type key struct {
		projectID int64
		mrIID     int64
	}
	counts := make(map[key]int)
	var order []key
	for _, e := range events {
		if e.Note.NoteableType != "MergeRequest" || e.Note.System {
			continue
		}
		k := key{projectID: e.ProjectID, mrIID: e.Note.NoteableIID}
		if _, seen := counts[k]; !seen {
			order = append(order, k)
		}
		counts[k]++
	}

	var out []mrCandidate
	for _, k := range order {
		if len(out) >= max {
			break
		}
		out = append(out, mrCandidate{projectID: k.projectID, mrIID: k.mrIID, eventCount: counts[k]})
	}
	return out
}

// countUserNotes counts, over discussions, the notes authored by userID
// that are not system notes and fall inside window (the exact count rule
// of TZ.md section 5.4). Every note type counts -- DiscussionNote,
// DiffNote, and the untyped plain comment -- because the undercount this
// probe hunts for is not specific to thread replies: TZ.md section 5.5.3
// only needs a real note count to compare a raw event count against,
// whatever kind of note it is.
func countUserNotes(discussions []domain.Discussion, userID int64, window domain.DateRange) (exact int) {
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.Author.ID != userID || n.System {
				continue
			}
			if !window.Contains(n.CreatedAt) {
				continue
			}
			exact++
		}
	}
	return exact
}
