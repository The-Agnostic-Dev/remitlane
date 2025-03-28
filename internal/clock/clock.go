// Package clock is the single place that knows Remitlane's business time
// zone. Daily limits, the reconciliation report and console dates all follow
// Africa/Lagos (WAT, UTC+1) whatever zone the server runs in.
package clock

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Clock tells business time.
type Clock struct {
	loc *time.Location
	now func() time.Time
}

// New loads the named zone.
func New(tz string) (*Clock, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("clock: load %q: %w", tz, err)
	}
	return &Clock{loc: loc, now: time.Now}, nil
}

// Fixed returns a clock frozen at t, for tests.
func Fixed(t time.Time, loc *time.Location) *Clock {
	return &Clock{loc: loc, now: func() time.Time { return t }}
}

// Location is the business time zone.
func (c *Clock) Location() *time.Location { return c.loc }

// Now is the current time in the business zone.
func (c *Clock) Now() time.Time { return c.now().In(c.loc) }

// Today is the current business date, "2006-01-02".
func (c *Clock) Today() string { return c.Now().Format(dateLayout) }

// Yesterday is the previous business date.
func (c *Clock) Yesterday() string { return c.Now().AddDate(0, 0, -1).Format(dateLayout) }


// DateBounds returns the bounds of a business date given as "2006-01-02".
func (c *Clock) DateBounds(date string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation(dateLayout, date, c.loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("clock: bad date %q", date)
	}
	return start, start.AddDate(0, 0, 1), nil
}

// ValidDate reports whether s is a "2006-01-02" date.
func ValidDate(s string) bool {
	_, err := time.Parse(dateLayout, s)
	return err == nil
}

// Display renders t for the console: "6 Nov 2026, 14:05".
func (c *Clock) Display(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(c.loc).Format("2 Jan 2006, 15:04")
}

// DisplayDate renders a "2006-01-02" date as "Thu 5 Nov 2026".
func DisplayDate(date string) string {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return date
	}
	return t.Format("Mon 2 Jan 2006")
}
