package display

import "time"

// Schedule is a daily screen-off window in minutes after midnight. Until <
// From wraps past midnight (e.g. 23:00–07:00).
type Schedule struct {
	Enabled bool
	From    int
	Until   int
	Loc     *time.Location // nil means time.Local
}

// Contains reports whether t falls inside the off window.
func (s Schedule) Contains(t time.Time) bool {
	if !s.Enabled || s.From == s.Until {
		return false
	}
	if s.Loc != nil {
		t = t.In(s.Loc)
	}
	m := t.Hour()*60 + t.Minute()
	if s.From < s.Until {
		return m >= s.From && m < s.Until
	}
	return m >= s.From || m < s.Until
}
