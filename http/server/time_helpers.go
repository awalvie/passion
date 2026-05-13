package web

import (
	"fmt"
	"time"

	"passion/db"
)

func localDate(t time.Time) time.Time {
	return db.LocalDate(t)
}

func localDateKey(t time.Time) string {
	return localDate(t).Format("2006-01-02")
}

// daySuffix returns the English ordinal suffix for a day number (st, nd, rd, th).
func daySuffix(day int) string {
	switch {
	case day == 11 || day == 12 || day == 13:
		return "th"
	case day%10 == 1:
		return "st"
	case day%10 == 2:
		return "nd"
	case day%10 == 3:
		return "rd"
	default:
		return "th"
	}
}

// dayWithSuffix formats a time as "Jan 2nd".
func dayWithSuffix(t time.Time) string {
	d := t.Day()
	return t.Format("Jan") + " " + fmt.Sprintf("%d%s", d, daySuffix(d))
}

// relativeDateLabel returns "Today", "Yesterday", or "Mon Jan 2" for a given time.
func relativeDateLabel(t time.Time) string {
	today := localDate(time.Now())
	d := localDate(t)
	switch {
	case d.Equal(today):
		return "Today"
	case d.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return t.Format("Mon Jan 2")
	}
}

// mondayOfLocalDate returns the local Monday for the week containing t.
func mondayOfLocalDate(t time.Time) time.Time {
	t = localDate(t)
	// time.Weekday: Sunday=0, Monday=1, ... Saturday=6
	wd := int(t.Weekday())
	// Convert to days since Monday: Mon->0, Tue->1, ... Sun->6
	daysSinceMonday := (wd + 6) % 7
	return t.AddDate(0, 0, -daysSinceMonday)
}
