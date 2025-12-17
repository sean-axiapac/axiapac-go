package model

import "time"

type ClockinRecord struct {
	ID        string `json:"id"`
	Tag       string `json:"tag"`
	Date      string `json:"date"`
	Kind      string `json:"kind"`
	Timestamp string `json:"timestamp"`
	DeviceID  string `json:"device_id"`
	CardID    string `json:"card_id"`

	Status  string `json:"_status"`
	Changed string `json:"_changed"`

	// New fields
	ProcessStatus string    `json:"process_status"`
	CreatedAt     time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP;<-:create"`
	UpdatedAt     time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP on update CURRENT_TIMESTAMP"`
}

func (ClockinRecord) TableName() string {
	return "oktedi_records"
}
