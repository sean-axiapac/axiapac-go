package core

import (
	"time"

	"axiapac.com/axiapac/core/models"
)

// ValidateRoster inspects an employee's roster data and reports three things:
//
//   - isRoster: whether the employee is a roster employee at all — i.e. any
//     roster value is set (a roster time type and/or a roster start date).
//   - valid: whether the roster setup is internally consistent. A non-roster
//     employee is trivially valid. A roster employee is valid only when BOTH
//     anchor fields are present (a half-filled roster — time type without start
//     date, or vice versa — is invalid), the time type resolves, and an on/off
//     cycle is configured.
//   - reason: when valid is false, a human-readable explanation of why.
//
// This is intentionally separate from IsRosteredOn, which stays a pure cycle
// calculation. Callers use this to surface misconfiguration instead of silently
// treating an incompletely-configured employee as always rostered on.
func ValidateRoster(emp models.Employee, timeType *models.PayrollTimeType) (isRoster bool, valid bool, reason string) {
	hasTimeType := emp.RosterPayrollTimeTypeID != 0
	hasStartDate := !emp.RosterStartDate.IsZero()

	// No roster values set at all — not a roster employee, trivially valid.
	if !hasTimeType && !hasStartDate {
		return false, true, ""
	}

	// Some roster value is set, so this is a roster employee. Both anchor
	// fields must now be present together.
	if !hasTimeType {
		return true, false, "roster start date set but roster payroll time type not set"
	}
	if !hasStartDate {
		return true, false, "roster payroll time type set but roster start date not set"
	}
	if timeType == nil {
		return true, false, "roster time type not found"
	}
	if timeType.RosteredDaysOn == 0 && timeType.RosteredDaysOff == 0 {
		return true, false, "roster cycle (days on/off) not configured"
	}
	return true, true, ""
}

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
		return false // date before roster start — cycle hasn't begun, not rostered on
	}
	dayInCycle := daysSinceStart % cycleLength
	return dayInCycle < int(daysOn)
}
