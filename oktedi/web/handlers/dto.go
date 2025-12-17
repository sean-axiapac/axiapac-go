package handlers

import "time"

// WatermelonPush represents the push payload to WatermelonDB
type WatermelonPush struct {
	Changes      Changes `json:"changes"`
	LastPulledAt int64   `json:"lastPulledAt"`
}

type Changes struct {
	Records Records `json:"records"`
}

type EmployeeClockInRecord struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	Kind      string    `json:"kind"`
	Timestamp time.Time `json:"timestamp"`
	CardID    string    `json:"cardId"`
	DeviceID  string    `json:"deviceId"`

	Status  string `json:"_status"`
	Changed string `json:"_changed"`
}

type Records struct {
	Created []EmployeeClockInRecord `json:"created"`
	Updated []EmployeeClockInRecord `json:"updated"`
	Deleted []EmployeeClockInRecord `json:"deleted"`
}
