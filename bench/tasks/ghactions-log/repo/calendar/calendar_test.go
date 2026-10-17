package calendar

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestNextBusinessDay(t *testing.T) {
	cases := [][2]string{
		{"2026-10-12", "2026-10-12"},
		{"2026-10-16", "2026-10-16"},
		{"2026-10-17", "2026-10-19"},
		{"2026-10-18", "2026-10-19"},
		{"2026-10-31", "2026-11-02"},
	}
	for _, c := range cases {
		if got := NextBusinessDay(day(c[0])).Format("2006-01-02"); got != c[1] {
			t.Errorf("NextBusinessDay(%s) = %s (%s), want %s", c[0], got, day(got).Weekday(), c[1])
		}
	}
}

func TestDueDate(t *testing.T) {
	cases := []struct {
		issued string
		terms  int
		want   string
	}{
		{"2026-10-01", 30, "2026-11-02"},
		{"2026-10-05", 14, "2026-10-19"},
		{"2026-10-02", 15, "2026-10-19"},
	}
	for _, c := range cases {
		if got := DueDate(day(c.issued), c.terms).Format("2006-01-02"); got != c.want {
			t.Errorf("DueDate(%s, net %d) = %s, want %s", c.issued, c.terms, got, c.want)
		}
	}
}
