// Package stats computes aggregates over an already-decoded artifact of any
// kind: total row count, a per-merge-request breakdown, a per-label
// breakdown, and a per-day breakdown (TZ.md section 7.2.1). It makes no API
// calls and does not read or write artifact files itself.
package stats
