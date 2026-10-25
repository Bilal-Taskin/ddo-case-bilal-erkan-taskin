package dateutil

import (
	"time"
)

// IsLeapYear returns true if the specified year is a leap year.
func IsLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// DaysInMonth returns the total number of days in the specified month and year.
func DaysInMonth(year int, month time.Month) int {
	switch month {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		if IsLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 31
	}
}

// AddClampedMonths adds the given number of months to t without month-overflow.
// Standard Go time.AddDate(0, months, 0) overflows 31 Jan + 1 month into 2 or 3 March.
// AddClampedMonths clamps the day to the last day of the target month (e.g. 31 Jan -> 28/29 Feb).
func AddClampedMonths(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	totalMonths := y*12 + int(m) - 1 + months

	newYear := totalMonths / 12
	newMonthIndex := totalMonths % 12
	if newMonthIndex < 0 {
		newMonthIndex += 12
		newYear--
	}
	newMonth := time.Month(newMonthIndex + 1)

	maxDays := DaysInMonth(newYear, newMonth)
	newDay := d
	if newDay > maxDays {
		newDay = maxDays
	}

	return time.Date(newYear, newMonth, newDay, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// AddDays adds the given number of days to t preserving the location.
func AddDays(t time.Time, days int) time.Time {
	return t.AddDate(0, 0, days)
}

// TruncateToDate resets hour, minute, second, nanosecond to UTC midnight.
func TruncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// DaysBetween returns the number of calendar days between start and end (end - start).
// Positive if end > start, negative if end < start.
func DaysBetween(start, end time.Time) int {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	diff := e.Sub(s)
	return int(diff.Hours() / 24)
}
