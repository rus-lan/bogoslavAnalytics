// Package filter applies deterministic, pure filters to artifact rows
// already held in memory: date range, group, project, single-MR point
// mode, comment-count threshold, and semantic label (TZ.md section 4 and
// section 7.2). It makes no HTTP calls, calls no LLM, and never reads or
// writes artifact files itself.
package filter
