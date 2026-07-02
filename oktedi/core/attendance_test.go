package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/core/models"
	"github.com/stretchr/testify/assert"
)

// A 5-on/2-off roster starting Mon 2026-01-05. Days in cycle 0..4 are ON
// (Mon–Fri), 5..6 OFF (Sat–Sun).
var fiveTwo = models.PayrollTimeType{PayrollTimeTypeID: 10, RosteredDaysOn: 5, RosteredDaysOff: 2}

func rosterEmp() models.Employee {
	return models.Employee{
		EmployeeID:              1,
		RosterPayrollTimeTypeID: 10,
		RosterStartDate:         time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		IdentificationTag:       "T1",
	}
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCountConsecutiveAbsent(t *testing.T) {
	emp := rosterEmp()
	tt := &fiveTwo

	t.Run("absent every scheduled day → counts back over weekend gap", func(t *testing.T) {
		// Never present. From Fri 2026-01-16 back: Fri16,Thu15,Wed14,Tue13,Mon12,
		// (Sun11/Sat10 OFF, skipped) Fri09,Thu08,Wed07,Tue06,Mon05 = 10 scheduled
		// days absent. Roster starts Jan 5, so before that IsRosteredOn=false.
		none := func(time.Time) bool { return false }
		got := CountConsecutiveAbsent(emp, tt, day(2026, 1, 16), none, AbsentLookbackDays)
		assert.Equal(t, 10, got)
	})

	t.Run("record on a scheduled day breaks the streak", func(t *testing.T) {
		// Present only on Wed 2026-01-14. From Fri16: Fri16,Thu15 absent (2), then
		// Wed14 has a record → stop.
		present := map[string]bool{"2026-01-14": true}
		has := func(d time.Time) bool { return present[d.Format("2006-01-02")] }
		got := CountConsecutiveAbsent(emp, tt, day(2026, 1, 16), has, AbsentLookbackDays)
		assert.Equal(t, 2, got)
	})

	t.Run("present on the viewed day → zero", func(t *testing.T) {
		present := map[string]bool{"2026-01-16": true}
		has := func(d time.Time) bool { return present[d.Format("2006-01-02")] }
		got := CountConsecutiveAbsent(emp, tt, day(2026, 1, 16), has, AbsentLookbackDays)
		assert.Equal(t, 0, got)
	})

	t.Run("weekend viewed day is skipped, streak continues from prior Friday", func(t *testing.T) {
		// From Sat 2026-01-17 (OFF): skip Sat/Sun, then Fri16..Mon12 (5) absent,
		// weekend skip, Fri09..Mon05 (5) = 10. Never present.
		none := func(time.Time) bool { return false }
		got := CountConsecutiveAbsent(emp, tt, day(2026, 1, 17), none, AbsentLookbackDays)
		assert.Equal(t, 10, got)
	})
}

func TestCountTotalAbsent(t *testing.T) {
	emp := rosterEmp()
	tt := &fiveTwo

	t.Run("counts all scheduled misses in window, not just consecutive", func(t *testing.T) {
		// today = Fri 2026-01-16. Present Wed14 and Mon12. Scheduled days in range
		// back to roster start Jan5: Mon05..Fri09 (5) + Mon12..Fri16 (5) = 10
		// scheduled; minus 2 present = 8 absent.
		present := map[string]bool{"2026-01-14": true, "2026-01-12": true}
		has := func(d time.Time) bool { return present[d.Format("2006-01-02")] }
		got := CountTotalAbsent(emp, tt, day(2026, 1, 16), has, AbsentLookbackDays)
		assert.Equal(t, 8, got)
	})

	t.Run("fully present → zero", func(t *testing.T) {
		all := func(time.Time) bool { return true }
		got := CountTotalAbsent(emp, tt, day(2026, 1, 16), all, AbsentLookbackDays)
		assert.Equal(t, 0, got)
	})
}

func TestNonRosterEmployeeAlwaysOn(t *testing.T) {
	// No roster time type → IsRosteredOn is always true, so every day in the
	// window with no record counts. Guarding this documents the "always on"
	// fail-open behaviour the dashboard inherits from IsRosteredOn.
	emp := models.Employee{EmployeeID: 2, IdentificationTag: "T2"}
	none := func(time.Time) bool { return false }
	// 7-day lookback for a compact assertion.
	got := CountConsecutiveAbsent(emp, nil, day(2026, 1, 16), none, 6)
	assert.Equal(t, 7, got) // days 16..10 inclusive
}
