package gitlab

import (
	"context"
	"errors"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

const (
	// smokeWindowDays is the recent window the smoke test samples (TZ.md
	// section 5.5.1: "дефолт: последние 90 дней").
	smokeWindowDays = 90
	// smokeMaxCandidates is how many MR candidates the smoke test samples
	// from the recent events (TZ.md section 5.5.2: "до 5 MR-кандидатов").
	smokeMaxCandidates = 5
)

// SmokeTest runs the DiscussionNote instance-capability detection
// described in TZ.md section 5.5: it checks whether this GitLab instance's
// Events API surfaces DiscussionNote (thread reply) comments, which
// upstream documentation contradicts itself about. Only the detection
// lives here; deciding what strategy the result should select is the
// responsibility of search/'s autoselector (TZ.md section 5.3).
//
// Procedure:
//  1. Fetch userID's comment events over the last smokeWindowDays days,
//     without a target_type filter.
//  2. Pick up to smokeMaxCandidates merge requests referenced by those
//     events and fetch their exact /discussions count over the same
//     window.
//  3. If any candidate's raw event count for that MR is lower than its
//     exact discussion count, the events API is losing DiscussionNote
//     replies: SmokeFailed.
//  4. If no sampled candidate had any thread-reply notes at all, the
//     result is inconclusive: SmokeUnknown (the caller must treat this as
//     equivalent to a failure and fall back to bruteforce, per TZ.md
//     section 5.5.4 -- that fallback decision itself belongs to search/).
//  5. Otherwise: SmokePassed.
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

	foundThreadReply := false
	for _, cand := range candidates {
		discussions, err := c.Discussions(ctx, cand.projectID, cand.mrIID)
		if err != nil && !errors.Is(err, ErrPageLimitReached) {
			return domain.SmokeUnknown, fmt.Errorf("gitlab: smoke test discussions: %w", err)
		}

		exact, threadReplies := countUserNotes(discussions, userID, window)
		if threadReplies == 0 {
			continue
		}
		foundThreadReply = true
		if cand.eventCount < exact {
			return domain.SmokeFailed, nil
		}
	}
	if !foundThreadReply {
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
// of TZ.md section 5.4), and separately counts how many of those are
// thread replies (Type == domain.NoteTypeDiscussion).
func countUserNotes(discussions []domain.Discussion, userID int64, window domain.DateRange) (exact, threadReplies int) {
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.Author.ID != userID || n.System {
				continue
			}
			if !window.Contains(n.CreatedAt) {
				continue
			}
			exact++
			if n.Type == domain.NoteTypeDiscussion {
				threadReplies++
			}
		}
	}
	return exact, threadReplies
}
