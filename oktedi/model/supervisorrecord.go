package model

import "time"

type SupervisorRecord struct {
	ID           int32      `gorm:"primaryKey;column:id" json:"id"`
	SupervisorId int        `json:"supervisorId"`
	EmployeeId   int        `json:"employeeId"`
	Project      string     `json:"project"`
	Wbs          string     `json:"wbs"`
	Date         string     `json:"date"`
	Clockin      *time.Time `json:"clockin"`
	Clockout     *time.Time `json:"clockout"`
	DeviceID     string     `json:"deviceId"`

	CreatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP;<-:create" json:"createdAt"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP on update CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (SupervisorRecord) TableName() string {
	return "oktedi_supervisor_records"
}
