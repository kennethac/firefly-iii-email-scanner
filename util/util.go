package util

import "time"

// Map applies a function to each element of a slice and returns a new slice with the results.
func Map[T, V any](ts []T, fn func(T) V) []V {
	result := make([]V, len(ts))
	for i, t := range ts {
		result[i] = fn(t)
	}
	return result
}

// Reports whether the two times are on the same year, month, and day.
func SameDay(t1 time.Time, t2 time.Time) bool {
	return t1.Year() == t2.Year() &&
		t1.Month() == t2.Month() &&
		t1.Day() == t2.Day()
}

// Reports whether the two times are 'close' to each other, within a
// certain upper and lower threshold of days difference.
func CloseDay(t1 time.Time, t2 time.Time, lessThreshold int, greaterThreshold int) bool {
	for i := -lessThreshold; i <= greaterThreshold; i++ {
		if SameDay(t1, t2.AddDate(0, 0, i)) {
			return true
		}
	}
	return false
}
