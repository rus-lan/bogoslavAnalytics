package search

import "time"

// Options configures the strategy autoselector (TZ.md section 5.3) beyond
// what Params carries.
type Options struct {
	// Strict forces bruteforce regardless of range age or smoke result
	// (TZ.md section 5.3c). This is the only strategy choice a user makes
	// directly; everything else is decided by SelectStrategy.
	Strict bool

	// RetentionYears is how many years back GitLab keeps user activity
	// events (TZ.md sections 5.3a, 5.6.3). A range whose start is older
	// than this always falls back to bruteforce. Zero means
	// DefaultRetentionYears. TZ.md section 14.2 leaves the exact GitLab
	// version and hence the exact retention window undocumented, so this
	// is a configurable parameter rather than a hardcoded constant.
	RetentionYears int

	// Now returns the current time, for retention-window tests. Nil means
	// time.Now.
	Now func() time.Time
}

// retentionYears returns o.RetentionYears, or DefaultRetentionYears if it
// is not set.
func (o Options) retentionYears() int {
	if o.RetentionYears > 0 {
		return o.RetentionYears
	}
	return DefaultRetentionYears
}

// now returns o.Now, or time.Now if it is not set.
func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}
