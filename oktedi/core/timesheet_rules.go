package core

import (
	"fmt"
	"time"

	"axiapac.com/axiapac/core/models"
)

const (
	StartEarlyThreshold  = 15 * time.Minute
	StartLateThreshold   = 10 * time.Minute
	FinishEarlyThreshold = 10 * time.Minute
	FinishLateThreshold  = 15 * time.Minute
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

func ApplyStartRule(actual, defined time.Time) time.Time {
	// Rule: 15 min early or 10 min late -> use defined
	diff := actual.Sub(defined) // negative if early

	// Early: < -15 min? No, "15 min early ... use defined".
	// Meaning if I arrive 07:45 for 08:00 start (15m early), use 08:00?
	// The requirement: "if ... 15 min early or 10 min late ... use defined start time"
	// This usually implies a window. "Within 15m early and 10m late"?
	// Or "If > 15m early, use actual"?
	// "if actual start time is 15 min early or 10 min late ... use defined"
	// This usually means "If actual is WITHIN [Defined - 15m, Defined + 10m], snap to Defined".
	// Let's verify interpretation.
	// "15 min early" usually means "Time <= Defined - 15m".
	// BUT, if I am *very* early (1 hour), I probably want to get paid? Or is this about rounding "close enough" times?
	// "allowance". If I start at 7:50 (10m early), it snaps to 8:00.
	// If I start at 8:05 (5m late), it snaps to 8:00.
	// If I start at 7:30 (30m early), it stays 7:30?
	// "if ... 15 min early ... use defined" logic usually works like a tolerance window.
	// If Start >= Defined - 15m AND Start <= Defined + 10m -> Defined.
	// Else -> Actual.

	if diff >= -StartEarlyThreshold && diff <= StartLateThreshold {
		return defined
	}
	return actual
}

func ApplyFinishRule(actual, defined time.Time) time.Time {
	// Rule: 10 min early or 15 min late -> use defined
	// Window: [Defined - 10m, Defined + 15m] -> Defined
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
