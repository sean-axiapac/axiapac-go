package core

import (
	"time"

	"axiapac.com/axiapac/axiapac/v1/common/eraid"
	"axiapac.com/axiapac/core/models"
)

// employeeEndDateSentinel is the legacy "no end date" marker. Some Employee rows
// carry EndDate = 1900-01-01 instead of NULL to mean "not terminated"; both are
// treated as active. This matches the employee search endpoint's NotTerminated
// rule (EndDate IS NULL OR EndDate <= '1900-01-01' OR EndDate >= <date>).
var employeeEndDateSentinel = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

// onlyDate strips the time-of-day so terminations are compared at day
// granularity, mirroring SQL's date comparison (EndDate >= '2006-01-02').
func onlyDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ActiveEmployee is the single source of truth for whether an employee is a
// current, non-terminated record as of asOf. It is shared by the dashboard and
// the daily review (prepare) flows so all screens target the same set of staff.
//
// It gates *proactive* inclusion only — who we roster / create absent rows for.
// It must NOT gate attribution of existing clock-in records: work that actually
// happened is always recorded and paid regardless of active status, so the
// employee load stays complete and this predicate is applied at the decision
// points (injectAbsentRows, the dashboard roster pass), not at load time.
//
// Active means:
//   - current era (EraId == Present), and
//   - not terminated before asOf, where a NULL / <= 1900-01-01 EndDate means
//     "no end date" (still employed).
//
// asOf is the date being viewed/prepared, NOT necessarily today: viewing a past
// date must include people who were employed on that date even if they have
// since been terminated. Passing today reproduces the employee picker's rule.
func ActiveEmployee(emp models.Employee, asOf time.Time) bool {
	if emp.EraID != int32(eraid.Present) {
		return false
	}
	if emp.EndDate.IsZero() {
		return true // NULL end date
	}
	if !emp.EndDate.After(employeeEndDateSentinel) {
		return true // <= 1900-01-01 legacy sentinel
	}
	return !onlyDate(emp.EndDate).Before(onlyDate(asOf)) // EndDate >= asOf
}
