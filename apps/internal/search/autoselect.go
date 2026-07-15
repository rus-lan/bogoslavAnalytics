package search

import (
	"context"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// DefaultRetentionYears is the default events-table retention window
// GitLab documents (TZ.md sections 5.3a and 5.6.3): a range whose start is
// older than this many years always falls back to bruteforce. TZ.md
// section 14.2 leaves the exact GitLab version that introduced this
// undocumented, hence it is a default, not a hardcoded rule -- see
// Options.RetentionYears.
const DefaultRetentionYears = 3

// SelectStrategy is the strategy autoselector (TZ.md section 5.3): not a
// user choice, a function of inputs. It falls back to bruteforce when any
// of the following holds:
//
//   - p.Range.From is older than opts.RetentionYears (default
//     DefaultRetentionYears) years before opts.Now(): events that old are
//     pruned from GitLab's events table (TZ.md section 5.3a).
//   - opts.Strict is set (TZ.md section 5.3c).
//   - the DiscussionNote smoke test comes back SmokeFailed or
//     SmokeUnknown. TZ revision 2 made SmokeUnknown conservative on
//     purpose: an inconclusive smoke result means the events strategy may
//     be silently undercounting on this instance, in which case an
//     affected merge request never becomes a candidate and the exact
//     /discussions recount that would otherwise catch a wrong count never
//     even runs on it -- a silent false negative rather than a wrong
//     number (TZ.md section 5.5.4).
//
// The smoke test only runs when neither of the two free checks (range age,
// strict mode) already decided the answer, since it costs its own set of
// requests. When it does not run, the returned domain.SmokeResult is the
// zero value (empty string), which Query.Smoke's omitempty tag drops from
// the artifact.
func SelectStrategy(ctx context.Context, client Client, p Params, opts Options) (domain.Strategy, domain.SmokeResult, error) {
	if opts.Strict {
		return domain.StrategyBruteforce, "", nil
	}

	cutoffInstant := opts.now().AddDate(-opts.retentionYears(), 0, 0)
	cutoff := domain.NewDate(cutoffInstant.Year(), cutoffInstant.Month(), cutoffInstant.Day())
	if p.Range.From.Before(cutoff) {
		return domain.StrategyBruteforce, "", nil
	}

	smoke, err := client.SmokeTest(ctx, p.UserID)
	if err != nil {
		return "", "", fmt.Errorf("search: select strategy: smoke test: %w", err)
	}
	if smoke != domain.SmokePassed {
		return domain.StrategyBruteforce, smoke, nil
	}
	return domain.StrategyEvents, smoke, nil
}
