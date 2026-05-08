package core

import (
	"time"

	"axiapac.com/axiapac/core/models"
)

// IsRosteredOn returns true if the employee is expected to work on the given date.
// If timeType is nil (no roster assigned), the employee is always considered rostered on.
func IsRosteredOn(emp models.Employee, timeType *models.PayrollTimeType, date time.Time) bool {
	if timeType == nil {
		return true
	}
	if emp.RosterPayrollTimeTypeID == 0 {
		return true
	}
	if emp.RosterStartDate.IsZero() {
		return true
	}
	daysOn := timeType.RosteredDaysOn
	daysOff := timeType.RosteredDaysOff
	if daysOn == 0 && daysOff == 0 {
		return true
	}
	cycleLength := int(daysOn + daysOff)
	startDay := time.Date(emp.RosterStartDate.Year(), emp.RosterStartDate.Month(), emp.RosterStartDate.Day(), 0, 0, 0, 0, time.UTC)
	targetDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	daysSinceStart := int(targetDay.Sub(startDay).Hours() / 24)
	if daysSinceStart < 0 {
		return true // date before roster start — treat as rostered on
	}
	dayInCycle := daysSinceStart % cycleLength
	return dayInCycle < int(daysOn)
}
