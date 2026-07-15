package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDate_MarshalJSON_formatsDateOnly(t *testing.T) {
	cases := []struct {
		name string
		date Date
		want string
	}{
		{"first day of year", NewDate(2026, time.January, 1), `"2026-01-01"`},
		{"last day of year", NewDate(2026, time.December, 31), `"2026-12-31"`},
		{"single digit month and day padded", NewDate(2026, time.March, 5), `"2026-03-05"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.date)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDate_UnmarshalJSON_parsesDateOnly(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    Date
		wantErr bool
	}{
		{"valid date", `"2026-06-30"`, NewDate(2026, time.June, 30), false},
		{"invalid format", `"30-06-2026"`, Date{}, true},
		{"not a string", `20260630`, Date{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got Date
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("Unmarshal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDate_Start_returnsUTCMidnight(t *testing.T) {
	d := NewDate(2026, time.March, 1)
	want := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	if got := d.Start(); !got.Equal(want) {
		t.Errorf("Start() = %v, want %v", got, want)
	}
}

func TestDate_End_returnsEndOfDayMillisecond(t *testing.T) {
	d := NewDate(2026, time.June, 30)
	want := time.Date(2026, time.June, 30, 23, 59, 59, 999_000_000, time.UTC)
	if got := d.End(); !got.Equal(want) {
		t.Errorf("End() = %v, want %v", got, want)
	}
	if got := d.End().Format("15:04:05.000Z"); got != "23:59:59.999Z" {
		t.Errorf("End() formatted = %s, want 23:59:59.999Z", got)
	}
}

func TestNewDateRange_invalidWhenFromAfterTo(t *testing.T) {
	from := NewDate(2026, time.June, 30)
	to := NewDate(2026, time.January, 1)
	_, err := NewDateRange(from, to)
	if err == nil {
		t.Fatal("NewDateRange() error = nil, want ErrInvalidDateRange")
	}
}

func TestNewDateRange_validWhenFromBeforeOrEqualTo(t *testing.T) {
	cases := []struct {
		name string
		from Date
		to   Date
	}{
		{"from before to", NewDate(2026, time.January, 1), NewDate(2026, time.June, 30)},
		{"from equal to", NewDate(2026, time.January, 1), NewDate(2026, time.January, 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewDateRange(tc.from, tc.to); err != nil {
				t.Errorf("NewDateRange() error = %v, want nil", err)
			}
		})
	}
}

func TestDateRange_Contains_boundaryInstants(t *testing.T) {
	from := NewDate(2026, time.January, 1)
	to := NewDate(2026, time.June, 30)
	r, err := NewDateRange(from, to)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	startInstant := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endInstant := time.Date(2026, time.June, 30, 23, 59, 59, 999_000_000, time.UTC)

	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"exactly start instant is included", startInstant, true},
		{"exactly end instant is included", endInstant, true},
		{"one millisecond before start is excluded", startInstant.Add(-time.Millisecond), false},
		{"one millisecond after end is excluded", endInstant.Add(time.Millisecond), false},
		{"mid range is included", time.Date(2026, time.March, 15, 12, 30, 0, 0, time.UTC), true},
		{"non-UTC instant is converted before comparison", startInstant.In(time.FixedZone("UTC+2", 2*60*60)), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r.Contains(tc.t); got != tc.want {
				t.Errorf("Contains(%v) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}

func TestDateRange_Contains_singleDayRange(t *testing.T) {
	d := NewDate(2026, time.March, 1)
	r, err := NewDateRange(d, d)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}
	inside := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	outside := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	if !r.Contains(inside) {
		t.Errorf("Contains(%v) = false, want true", inside)
	}
	if r.Contains(outside) {
		t.Errorf("Contains(%v) = true, want false", outside)
	}
}
