package display

import "time"

// Rule switches the panel on (On) or off at Time ("HH:MM") on the selected
// Days: "daily", "weekdays" (Mon-Fri) or "weekend" (Sat-Sun). Validated by config;
// a rule with an unparseable time or unknown days never fires.
type Rule struct {
	Time string
	On   bool
	Days string
}

// Schedule is a set of rules evaluated in Location (nil = time.Local). The panel
// follows the most recent rule that has fired; no rules means no schedule.
type Schedule struct {
	Rules    []Rule
	Location *time.Location
}

func (r Rule) matchesDay(d time.Weekday) bool {
	switch r.Days {
	case "daily":
		return true
	case "weekdays":
		return d != time.Saturday && d != time.Sunday
	case "weekend":
		return d == time.Saturday || d == time.Sunday
	}
	return false
}

// Off reports whether the schedule holds the panel off at t: the latest rule
// that fired at or before t, looking back a week, is an "off" rule.
func (s Schedule) Off(t time.Time) bool {
	if len(s.Rules) == 0 {
		return false
	}
	loc := s.Location
	if loc == nil {
		loc = time.Local
	}
	t = t.In(loc)
	var latest time.Time
	off := false
	for _, r := range s.Rules {
		hm, err := time.Parse("15:04", r.Time)
		if err != nil {
			continue
		}
		for back := 0; back <= 7; back++ {
			day := t.AddDate(0, 0, -back)
			if !r.matchesDay(day.Weekday()) {
				continue
			}
			at := time.Date(day.Year(), day.Month(), day.Day(), hm.Hour(), hm.Minute(), 0, 0, loc)
			if at.After(t) {
				continue
			}
			// Ties (two rules at the same minute): "off" wins.
			if at.After(latest) || (at.Equal(latest) && !r.On) {
				latest, off = at, !r.On
			}
			break
		}
	}
	return off
}
