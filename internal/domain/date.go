package domain

import (
	"fmt"
	"time"
)

// dateLayout is the wire format for date-only values: YYYY-MM-DD.
const dateLayout = "2006-01-02"

// Date is a date-only value (no time-of-day, no zone) that is carried on
// the wire as "YYYY-MM-DD". It is used for the from/to query bounds.
type Date struct {
	year  int
	month time.Month
	day   int
}

// NewDate builds a Date from a calendar year, month and day.
func NewDate(year int, month time.Month, day int) Date {
	return Date{year: year, month: month, day: day}
}

// ParseDate parses a "YYYY-MM-DD" string into a Date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return NewDate(t.Year(), t.Month(), t.Day()), nil
}

// String renders the date as "YYYY-MM-DD".
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.year, int(d.month), d.day)
}

// Start returns the inclusive UTC start instant of the date: 00:00:00.000Z.
func (d Date) Start() time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

// End returns the inclusive UTC end instant of the date: 23:59:59.999Z.
func (d Date) End() time.Time {
	return time.Date(d.year, d.month, d.day, 23, 59, 59, 999_000_000, time.UTC)
}

// Before reports whether d is strictly before other.
func (d Date) Before(other Date) bool {
	return d.Start().Before(other.Start())
}

// After reports whether d is strictly after other.
func (d Date) After(other Date) bool {
	return d.Start().After(other.Start())
}

// MarshalJSON renders the date as a "YYYY-MM-DD" JSON string.
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON parses a "YYYY-MM-DD" JSON string into the date.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return fmt.Errorf("parse date: not a JSON string: %s", s)
	}
	parsed, err := ParseDate(s[1 : len(s)-1])
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// DateRange is an inclusive [from, to] range of calendar dates. It expands
// to the inclusive UTC instant range [from T00:00:00.000Z, to
// T23:59:59.999Z], per TZ.md section 5.4.
type DateRange struct {
	From Date `json:"from"`
	To   Date `json:"to"`
}

// NewDateRange builds a DateRange, rejecting a from date after the to date.
func NewDateRange(from, to Date) (DateRange, error) {
	if from.After(to) {
		return DateRange{}, ErrInvalidDateRange
	}
	return DateRange{From: from, To: to}, nil
}

// Start returns the inclusive UTC start instant of the range.
func (r DateRange) Start() time.Time {
	return r.From.Start()
}

// End returns the inclusive UTC end instant of the range.
func (r DateRange) End() time.Time {
	return r.To.End()
}

// Contains reports whether t falls within the inclusive UTC instant range
// [Start(), End()]. t is converted to UTC before comparison.
func (r DateRange) Contains(t time.Time) bool {
	t = t.UTC()
	start, end := r.Start(), r.End()
	return !t.Before(start) && !t.After(end)
}
