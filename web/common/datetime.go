package common

import (
	"encoding/json"
	"time"
)

type LocalDateTime struct {
	time.Time
}

const dateTimeLayout = "2006-01-02T15:04:05"

func (l *LocalDateTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		l.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(dateTimeLayout, s)
	if err != nil {
		return err
	}
	l.Time = t
	return nil
}

func (l LocalDateTime) MarshalJSON() ([]byte, error) {
	if l.Time.IsZero() {
		return json.Marshal("")
	}
	return json.Marshal(l.Format(dateTimeLayout))
}
