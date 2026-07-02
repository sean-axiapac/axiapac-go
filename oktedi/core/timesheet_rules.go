package core

import (
	"fmt"
	"time"

	"axiapac.com/axiapac/core/models"
)

const (
	// Any clock-in before the set start snaps to it (no early cap, so early
	// arrivals never become morning overtime); a clock-in up to StartLateThreshold
	// after the set start also snaps to it.
	StartLateThreshold   = 15 * time.Minute // within 15m after set start  → use set start
	FinishEarlyThreshold = 15 * time.Minute // within 15m before set finish → use set finish
	FinishLateThreshold  = 15 * time.Minute // within 15m after set finish  → use set finish
)

// AdjustTimesheetResult holds the adjusted times and any applied rules metadata if needed
type AdjustTimesheetResult struct {
	StartTime  time.Time
	FinishTime time.Time
}

// WorkHourDefinition simplifies the passing of work hour data
type WorkHourDefinition struct {
	Start  string
	Finish string
	Break  int32
}

// AdjustTimesheetHours applies the business rules to adjust the start and finish times
// based on the defined work hours for the employee.
func AdjustTimesheetHours(
	actualStart, actualFinish time.Time,
	emp models.Employee,
	empWorkHours map[int32]map[int32]models.EmployeeWorkHour,
	regionWorkHours map[int32]map[int32]models.RegionWorkHour,
) (AdjustTimesheetResult, error) {

	// 1. Determine standard work hours for the day
	def, found := GetDefinedWorkHours(actualStart, emp, empWorkHours, regionWorkHours)
	if !found {
		// No defined hours, return actuals
		return AdjustTimesheetResult{StartTime: actualStart, FinishTime: actualFinish}, nil
	}

	// 2. Parse defined times
	// The defined Start/Finish are strings like "08:00". We need to combine them with the actual date.
	// Assuming the work hours apply to the date of the actual start.
	dateBase := time.Date(actualStart.Year(), actualStart.Month(), actualStart.Day(), 0, 0, 0, 0, actualStart.Location())

	defStart, err := ParseTimeOnDate(dateBase, def.Start)
	if err != nil {
		return AdjustTimesheetResult{StartTime: actualStart, FinishTime: actualFinish}, fmt.Errorf("invalid defined start time %s: %w", def.Start, err)
	}

	defFinish, err := ParseTimeOnDate(dateBase, def.Finish)
	if err != nil {
		return AdjustTimesheetResult{StartTime: actualStart, FinishTime: actualFinish}, fmt.Errorf("invalid defined finish time %s: %w", def.Finish, err)
	}

	// Adjust for day crossings if necessary (e.g. night shifts).
	// If finish is before start, assume next day.
	if defFinish.Before(defStart) {
		defFinish = defFinish.Add(24 * time.Hour)
	}

	// Also adjust actual finish if it looks like it crossed midnight relative to start,
	// handled by the caller usually, but here we just deal with time values.
	// The actualStart/Finish are passed as full time.Time.

	// 3. Apply comparison rules
	// skip rule if actual start time and actual finish time are the same
	if actualStart.Equal(actualFinish) {
		return AdjustTimesheetResult{StartTime: actualStart, FinishTime: actualFinish}, nil
	}

	finalStart := ApplyStartRule(actualStart, defStart)
	finalFinish := ApplyFinishRule(actualFinish, defFinish)

	return AdjustTimesheetResult{
		StartTime:  finalStart,
		FinishTime: finalFinish,
	}, nil
}

// GetDefinedWorkHours finds the applicable work hours for an employee on a specific day.
func GetDefinedWorkHours(
	date time.Time,
	emp models.Employee,
	empWorkHours map[int32]map[int32]models.EmployeeWorkHour, // EmpID -> DayOfWeek -> WorkHour
	regionWorkHours map[int32]map[int32]models.RegionWorkHour, // RegionID -> DayOfWeek -> WorkHour
) (WorkHourDefinition, bool) {
	dayOfWeek := int32(date.Weekday())

	// 1. Check if using Calendar/Region hours
	if emp.UseCalendarWorkHours {
		if regionMap, ok := regionWorkHours[emp.CalendarRegionID]; ok {
			if wh, ok := regionMap[dayOfWeek]; ok {
				return WorkHourDefinition{
					Start:  wh.Start,
					Finish: wh.Finish,
					Break:  wh.Break,
				}, true
			}
		}
		return WorkHourDefinition{}, false
	}

	// 2. Use Employee Personal Hours
	if empMap, ok := empWorkHours[emp.EmployeeID]; ok {
		if wh, ok := empMap[dayOfWeek]; ok {
			return WorkHourDefinition{
				Start:  wh.Start,
				Finish: wh.Finish,
				Break:  wh.Break,
			}, true
		}
	}

	return WorkHourDefinition{}, false
}

// GetBreakMinutes returns the break duration in minutes if defined in the work hours.
// Returns nil if no work hours are found.
func GetBreakMinutes(
	date time.Time,
	emp models.Employee,
	empWorkHours map[int32]map[int32]models.EmployeeWorkHour,
	regionWorkHours map[int32]map[int32]models.RegionWorkHour,
) *int32 {
	def, found := GetDefinedWorkHours(date, emp, empWorkHours, regionWorkHours)
	if !found {
		return nil
	}
	// Copy the value to return a pointer
	val := def.Break
	return &val
}

func ApplyStartRule(actual, defined time.Time) time.Time {
	// Any clock-in at or before the defined start — however early — counts as the
	// defined start (no morning overtime). A clock-in up to StartLateThreshold after
	// the defined start also snaps to it.
	if actual.Sub(defined) <= StartLateThreshold {
		return defined
	}
	return actual
}

func ApplyFinishRule(actual, defined time.Time) time.Time {
	// Snap to the set finish time when actual is within the tolerance window:
	// up to 15m before, or up to 15m after, the set finish time.
	diff := actual.Sub(defined)

	if diff >= -FinishEarlyThreshold && diff <= FinishLateThreshold {
		return defined
	}
	return actual
}

// ParseTimeOnDate combines a base date with a time string (e.g. "08:00")
func ParseTimeOnDate(baseDate time.Time, timeStr string) (time.Time, error) {
	// Try parsing standard formats
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		// Try with seconds
		t, err = time.Parse("15:04:05", timeStr)
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, baseDate.Location()), nil
}
