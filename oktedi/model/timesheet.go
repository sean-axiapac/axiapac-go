package model

import (
	"time"

	"axiapac.com/axiapac/core/models"
)

// type OktediTimesheet struct {
// 	ID           uint   `gorm:"primaryKey"`
// 	TimesheetID  uint   `gorm:"not null"`
// 	ReviewStatus string `gorm:"type:varchar(50);not null;"`

// 	// Relationships
// 	Timesheet models.Timesheet `gorm:"foreignKey:TimesheetID;references:TimesheetID"`
// }

type OktediTimesheet struct {
	ID           int32     `gorm:"primaryKey;column:id"`
	Date         time.Time `gorm:"column:date;type:date"`
	Hours        float64   `gorm:"column:hours;type:decimal(10,2)"`
	ReviewStatus string    `gorm:"column:review_status;type:varchar(50)"`
	Approved     bool      `gorm:"column:approved;type:bool;not null"`

	// Foreign Keys
	EmployeeID   int32  `gorm:"column:employee_id;not null"`
	TimesheetID  *int32 `gorm:"column:timesheet_id;null"`
	ProjectID    *int32 `gorm:"column:project_id;null"`
	CostCentreID *int32 `gorm:"column:cost_centre_id;null"`

	Timesheet  models.Timesheet     `gorm:"foreignKey:TimesheetID;references:TimesheetId"`
	Employee   models.Employee      `gorm:"foreignKey:EmployeeID;references:EmployeeId"`
	Project    models.Job           `gorm:"foreignKey:ProjectID;references:JobID"`
	CostCentre models.JobCostCentre `gorm:"foreignKey:CostCentreId;references:CostCentreID"`
}

func (OktediTimesheet) TableName() string {
	return "oktedi_timesheets"
}
