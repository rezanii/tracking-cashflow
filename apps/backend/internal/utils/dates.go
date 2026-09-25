package utils

import (
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

// Jakarta is the presentation timezone. Storage stays UTC; only period boundaries that a
// user picks ("this month") are resolved in local time.
var Jakarta = mustLoadJakarta()

func mustLoadJakarta() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	// Containers without tzdata still need a correct offset for WIB.
	return time.FixedZone("WIB", 7*60*60)
}

// ParseDate reads a YYYY-MM-DD value as a midnight UTC date, matching the DATE column.
func ParseDate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(DateLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must use the format YYYY-MM-DD")
	}
	return parsed, nil
}

func FormatDate(value time.Time) string {
	return value.Format(DateLayout)
}

// ResolveRange turns a named range into inclusive date bounds, evaluated in Jakarta time
// so "today" means today for the user rather than for the server.
func ResolveRange(name string) (time.Time, time.Time, bool) {
	now := time.Now().In(Jakarta)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch name {
	case "today":
		return today, today, true
	case "week":
		offset := (int(now.Weekday()) + 6) % 7 // Monday as the first day
		start := today.AddDate(0, 0, -offset)
		return start, start.AddDate(0, 0, 6), true
	case "month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, -1), true
	case "year":
		start := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(1, 0, -1), true
	}
	return time.Time{}, time.Time{}, false
}
