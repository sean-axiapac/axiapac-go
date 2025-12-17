package utils

import (
	"fmt"
	"time"
)

var BrisbaneTZ = time.FixedZone("UTC+10", 10*60*60)

func BrisbaneNow() time.Time {
	loc := time.FixedZone("AEST", 10*60*60) // Fallback to AEST if timezone load fails
	return time.Now().In(loc)
}

func MustParseDate(dateStr string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	return t
}

func ParseISOTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, fmt.Errorf("empty time string")
	}

	// Try standard RFC3339 format (ISO 8601)
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return &t, nil
	}

	// Try with nanoseconds (e.g. 2025-10-13T09:30:00.123Z)
	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return &t, nil
	}

	// Try fallback common formats
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if tt, e := time.ParseInLocation(layout, s, time.UTC); e == nil {
			return &tt, nil
		}
	}

	return nil, fmt.Errorf("failed to parse time: %v", s)
}
