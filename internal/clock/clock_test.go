package clock

import (
	"testing"
	"time"
)

func lagos(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestTodayFollowsLagos(t *testing.T) {
	loc := lagos(t)
	// 23:30 UTC on 5 Nov is 00:30 on 6 Nov in Lagos.
	c := Fixed(time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC), loc)
	if got := c.Today(); got != "2026-11-06" {
		t.Errorf("Today = %s", got)
	}
	if got := c.Yesterday(); got != "2026-11-05" {
		t.Errorf("Yesterday = %s", got)
	}
}

func TestDateBounds(t *testing.T) {
	loc := lagos(t)
	c := Fixed(time.Now(), loc)
	start, end, err := c.DateBounds("2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 2, 28, 23, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("start = %s, want %s", start.UTC(), want)
	}
	if end.Sub(start) != 24*time.Hour {
		t.Errorf("day is %s long", end.Sub(start))
	}
	if _, _, err := c.DateBounds("01/03/2026"); err == nil {
		t.Error("expected error")
	}
}

func TestDisplay(t *testing.T) {
	loc := lagos(t)
	c := Fixed(time.Now(), loc)
	got := c.Display(time.Date(2026, 11, 6, 13, 5, 0, 0, time.UTC))
	if got != "6 Nov 2026, 14:05" {
		t.Errorf("Display = %q", got)
	}
	if DisplayDate("2026-11-05") != "Thu 5 Nov 2026" {
		t.Errorf("DisplayDate = %q", DisplayDate("2026-11-05"))
	}
}
