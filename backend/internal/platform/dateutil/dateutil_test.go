package dateutil_test

import (
	"testing"
	"time"

	"fitgiris/internal/platform/dateutil"
)

func TestIsLeapYear(t *testing.T) {
	tests := []struct {
		year int
		want bool
	}{
		{2000, true},  // divisible by 400
		{1900, false}, // divisible by 100 but not 400
		{2020, true},  // divisible by 4
		{2024, true},  // divisible by 4
		{2025, false}, // not divisible by 4
		{2026, false}, // not divisible by 4
		{2400, true},  // divisible by 400
	}

	for _, tt := range tests {
		got := dateutil.IsLeapYear(tt.year)
		if got != tt.want {
			t.Errorf("IsLeapYear(%d) = %v; want %v", tt.year, got, tt.want)
		}
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2024, time.January, 31},
		{2024, time.February, 29}, // Leap year
		{2025, time.February, 28}, // Normal year
		{2024, time.March, 31},
		{2024, time.April, 30},
		{2024, time.May, 31},
		{2024, time.June, 30},
		{2024, time.July, 31},
		{2024, time.August, 31},
		{2024, time.September, 30},
		{2024, time.October, 31},
		{2024, time.November, 30},
		{2024, time.December, 31},
	}

	for _, tt := range tests {
		got := dateutil.DaysInMonth(tt.year, tt.month)
		if got != tt.want {
			t.Errorf("DaysInMonth(%d, %v) = %d; want %d", tt.year, tt.month, got, tt.want)
		}
	}
}

func TestAddClampedMonths(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		months   int
		expected time.Time
	}{
		{
			name:     "31 Jan 2024 + 1 month in leap year clamps to 29 Feb 2024",
			input:    time.Date(2024, time.January, 31, 10, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 Jan 2025 + 1 month in non-leap year clamps to 28 Feb 2025",
			input:    time.Date(2025, time.January, 31, 14, 30, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2025, time.February, 28, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "29 Feb 2024 + 12 months (1 year) clamps to 28 Feb 2025",
			input:    time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			months:   12,
			expected: time.Date(2025, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 March 2024 + 1 month clamps to 30 April 2024",
			input:    time.Date(2024, time.March, 31, 0, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, time.April, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 May 2024 + 1 month clamps to 30 June 2024",
			input:    time.Date(2024, time.May, 31, 0, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, time.June, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 August 2024 + 1 month clamps to 30 September 2024",
			input:    time.Date(2024, time.August, 31, 0, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, time.September, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 October 2024 + 1 month clamps to 30 November 2024",
			input:    time.Date(2024, time.October, 31, 0, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, time.November, 30, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "15 January + 1 month preserves day 15",
			input:    time.Date(2025, time.January, 15, 9, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2025, time.February, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 December 2024 + 2 months clamps to 28 Feb 2025",
			input:    time.Date(2024, time.December, 31, 0, 0, 0, 0, time.UTC),
			months:   2,
			expected: time.Date(2025, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 March 2024 - 1 month clamps to 29 Feb 2024",
			input:    time.Date(2024, time.March, 31, 0, 0, 0, 0, time.UTC),
			months:   -1,
			expected: time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "31 March 2025 - 1 month clamps to 28 Feb 2025",
			input:    time.Date(2025, time.March, 31, 0, 0, 0, 0, time.UTC),
			months:   -1,
			expected: time.Date(2025, time.February, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Multi-year: 29 Feb 2024 + 48 months (4 years) lands on 29 Feb 2028 (leap year)",
			input:    time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			months:   48,
			expected: time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dateutil.AddClampedMonths(tt.input, tt.months)
			if !got.Equal(tt.expected) {
				t.Errorf("AddClampedMonths(%v, %d) = %v; want %v", tt.input, tt.months, got, tt.expected)
			}
		})
	}
}

// TestGoAddDateOverflowDemonstration proves that Go's default AddDate overflows
// and dateutil.AddClampedMonths properly fixes this critical gym-membership bug.
func TestGoAddDateOverflowDemonstration(t *testing.T) {
	jan31_2025 := time.Date(2025, time.January, 31, 0, 0, 0, 0, time.UTC)

	// Standard Go behavior: overflows into March!
	goAddDateResult := jan31_2025.AddDate(0, 1, 0)
	if goAddDateResult.Month() != time.March {
		t.Fatalf("Standard Go AddDate expected to overflow to March, but got %v", goAddDateResult)
	}

	// Clamped behavior: accurately clamps to February 28!
	clampedResult := dateutil.AddClampedMonths(jan31_2025, 1)
	if clampedResult.Month() != time.February || clampedResult.Day() != 28 {
		t.Fatalf("Expected clamped result to be 2025-02-28, but got %v", clampedResult)
	}
}

func TestDaysBetween(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected int
	}{
		{
			name:     "Same day",
			start:    time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC),
			expected: 0,
		},
		{
			name:     "Next day",
			start:    time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC),
			expected: 1,
		},
		{
			name:     "January 31 to February 28 (non-leap year)",
			start:    time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			expected: 28,
		},
		{
			name:     "January 31 to February 29 (leap year)",
			start:    time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			expected: 29,
		},
		{
			name:     "30 days span",
			start:    time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 5, 31, 0, 0, 0, 0, time.UTC),
			expected: 30,
		},
		{
			name:     "Past date returns negative days",
			start:    time.Date(2025, 5, 31, 0, 0, 0, 0, time.UTC),
			end:      time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC),
			expected: -30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dateutil.DaysBetween(tt.start, tt.end)
			if got != tt.expected {
				t.Errorf("DaysBetween(%v, %v) = %d; want %d", tt.start, tt.end, got, tt.expected)
			}
		})
	}
}

func TestAddDays(t *testing.T) {
	start := time.Date(2025, 1, 30, 10, 0, 0, 0, time.UTC)
	got := dateutil.AddDays(start, 5)
	expected := time.Date(2025, 2, 4, 10, 0, 0, 0, time.UTC)

	if !got.Equal(expected) {
		t.Errorf("AddDays() = %v; want %v", got, expected)
	}
}
