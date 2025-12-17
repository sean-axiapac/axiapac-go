package common

import (
	"encoding/json"
	"fmt"
	"time"
)

type DateOnly struct {
	time.Time
}

const dateLayout = "2006-01-02" // yyyy-MM-dd

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	// b is a quoted string like `"2025-10-29"`
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	if s == "" {
		// handle empty date gracefully
		d.Time = time.Time{}
		return nil
	}

	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	d.Time = t
	return nil
}

func (d DateOnly) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return json.Marshal("")
	}
	return json.Marshal(d.Format(dateLayout))
}
