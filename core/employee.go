package core

import (
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Employee struct {
	EmployeeId                      uint   `gorm:"primaryKey;autoIncrement"`
	Code                            string `gorm:"uniqueIndex:idx_code_system_published"`
	PreferredName                   string
	FirstName                       string
	Surname                         string
	MiddleNames                     string
	TitleId                         int
	WorkPhoneId                     *int
	HomePhoneId                     *int
	OccupationId                    int
	DateOfBirth                     *time.Time
	Sex                             string `gorm:"size:1"`
	StartDate                       *time.Time
	EndDate                         *time.Time
	Email                           *string `gorm:"index"`
	EmailVerified                   bool
	EmailVerificationToken          *string
	EmailVerificationTokenCreatedAt *time.Time
	PrivateEmail                    *string
	LabourRateId                    *int
	IdentificationTag               *string
	Circa                           time.Time
	Picture                         *string
	DivisionId                      *int
	Status                          string
	PortableLeaveNo                 *string
	BloodType                       *string
	CheckLicenceExpiry              bool
	Apprentice                      bool
	Subcontractor                   bool
	ExcludeFromSchedule             bool
	VehicleId                       *int
	DepartmentId                    *int
	PositionId                      *int
	ReportsToId                     *int
	WorkSiteId                      *int
	CalendarRegionId                *int
	RosterPayrollTimeTypeId         *int
	RosterStartDate                 *time.Time
	JobId                           *int
	CostCentreId                    *int
	UseCalendarWorkHours            bool    `gorm:"default:true"`
	EraId                           int     `gorm:"default:1"`
	PayrollHours                    float64 `gorm:"type:decimal(13,4);default:0"`
	DataVersion                     int     `gorm:"default:1"`
	AddressId                       int
	Expatriate                      bool
	Attributes                      datatypes.JSON
	UncommittedId                   *int
	SystemPublished                 int `gorm:"default:1"`

	// GORM will auto-create relations if you define them
	// Example: Title Title `gorm:"foreignKey:TitleId;references:TitleId"`
}

func FindEmployeeByID(db *gorm.DB, id int) (*Employee, error) {
	var emp Employee
	result := db.First(&emp, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil // not found
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &emp, nil
}
