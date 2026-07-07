package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/axiapac/v1/common/eraid"
	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDate     = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	rosterOnDate = time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC) // day 4 from Jan 5 → ON for 5/2
)

func baseRefData(employees []models.Employee, ttMap map[int32]models.PayrollTimeType) *ReferenceData {
	empMap := make(map[int32]models.Employee)
	tagMap := make(map[string]models.Employee)
	for i := range employees {
		// Test employees are active unless a test sets EraID/EndDate explicitly.
		if employees[i].EraID == 0 {
			employees[i].EraID = int32(eraid.Present)
		}
		empMap[employees[i].EmployeeID] = employees[i]
		if employees[i].IdentificationTag != "" {
			tagMap[employees[i].IdentificationTag] = employees[i]
		}
	}
	return &ReferenceData{
		Employees:       employees,
		EmpMap:          empMap,
		TagMap:          tagMap,
		TimeTypeMap:     ttMap,
		EmpWorkHours:    map[int32]map[int32]models.EmployeeWorkHour{},
		RegionWorkHours: map[int32]map[int32]models.RegionWorkHour{},
	}
}

func TestShouldPreserveAbsent(t *testing.T) {
	tests := []struct {
		name     string
		existing model.OktediTimesheet
		expected bool
	}{
		{
			name:     "absent, Hours=0, not approved → safe to overwrite",
			existing: model.OktediTimesheet{ReviewStatus: "absent", Hours: 0, Approved: false},
			expected: false,
		},
		{
			name:     "absent, Hours=8, not approved → supervisor added hours → preserve",
			existing: model.OktediTimesheet{ReviewStatus: "absent", Hours: 8, Approved: false},
			expected: true,
		},
		{
			name:     "absent, Hours=0, approved → preserve",
			existing: model.OktediTimesheet{ReviewStatus: "absent", Hours: 0, Approved: true},
			expected: true,
		},
		{
			name:     "required, Hours=8 → not an absent row",
			existing: model.OktediTimesheet{ReviewStatus: "required", Hours: 8, Approved: false},
			expected: false,
		},
		{
			name:     "empty status, Hours=8 → not an absent row",
			existing: model.OktediTimesheet{ReviewStatus: "", Hours: 8, Approved: false},
			expected: false,
		},
		{
			name:     "accurate, Hours=8 → not an absent row",
			existing: model.OktediTimesheet{ReviewStatus: "accurate", Hours: 8, Approved: false},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, shouldPreserveAbsent(tc.existing))
		})
	}
}

func TestUpdateReviewStatusRosterGuard(t *testing.T) {
	rosterStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	fiveTwo := models.PayrollTimeType{PayrollTimeTypeID: 10, RosteredDaysOn: 5, RosteredDaysOff: 2}
	// testDate = Jan 10 = day 5 from Jan 5 → OFF for 5/2
	// rosterOnDate = Jan 9 = day 4 from Jan 5 → ON for 5/2

	// Active roster employee (unless a test overrides EraID/EndDate/roster).
	makeEmp := func(id int32, ttID int32) models.Employee {
		return models.Employee{
			EmployeeID: id, RosterPayrollTimeTypeID: ttID, RosterStartDate: rosterStart,
			EraID: int32(eraid.Present),
		}
	}
	makeRefData := func(emp models.Employee) *ReferenceData {
		return &ReferenceData{
			EmpMap:          map[int32]models.Employee{emp.EmployeeID: emp},
			TimeTypeMap:     map[int32]models.PayrollTimeType{10: fiveTwo},
			EmpWorkHours:    map[int32]map[int32]models.EmployeeWorkHour{},
			RegionWorkHours: map[int32]map[int32]models.RegionWorkHour{},
		}
	}

	t.Run("rostered OFF, timesheet with hours → not-rostered", func(t *testing.T) {
		emp := makeEmp(1, 10)
		ts := model.OktediTimesheet{EmployeeID: 1, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{1: ts}

		updateReviewStatus(testDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "not-rostered", timesheetMap[1].ReviewStatus)
	})

	t.Run("rostered OFF, existing required status → not-rostered wins", func(t *testing.T) {
		emp := makeEmp(2, 10)
		ts := model.OktediTimesheet{EmployeeID: 2, Hours: 8, ReviewStatus: "required"}
		timesheetMap := map[int32]model.OktediTimesheet{2: ts}

		updateReviewStatus(testDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "not-rostered", timesheetMap[2].ReviewStatus)
	})

	t.Run("rostered ON, absent row → absent unchanged", func(t *testing.T) {
		emp := makeEmp(3, 10)
		ts := model.OktediTimesheet{EmployeeID: 3, Hours: 0, ReviewStatus: "absent"}
		timesheetMap := map[int32]model.OktediTimesheet{3: ts}

		updateReviewStatus(rosterOnDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "absent", timesheetMap[3].ReviewStatus)
	})

	// on-cycle keys EmpWorkHours by the rosterOnDate weekday.
	onCycleStart := time.Date(2026, 1, 9, 8, 0, 0, 0, time.UTC) // rosterOnDate, 08:00
	onCycleDow := int32(onCycleStart.Weekday())

	t.Run("non-roster clock-in → not-rostered", func(t *testing.T) {
		emp := models.Employee{EmployeeID: 40, EraID: int32(eraid.Present)} // no roster config
		ts := model.OktediTimesheet{EmployeeID: 40, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{40: ts}
		refData := &ReferenceData{
			EmpMap:          map[int32]models.Employee{40: emp},
			TimeTypeMap:     map[int32]models.PayrollTimeType{},
			EmpWorkHours:    map[int32]map[int32]models.EmployeeWorkHour{},
			RegionWorkHours: map[int32]map[int32]models.RegionWorkHour{},
		}

		updateReviewStatus(testDate, timesheetMap, refData)

		// Matches the dashboard: a non-roster employee who clocks in is Not Rostered.
		assert.Equal(t, "not-rostered", timesheetMap[40].ReviewStatus)
	})

	t.Run("terminated clock-in (on-cycle) → not-rostered", func(t *testing.T) {
		emp := makeEmp(41, 10)
		emp.EndDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // terminated before the date
		ts := model.OktediTimesheet{EmployeeID: 41, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{41: ts}

		updateReviewStatus(rosterOnDate, timesheetMap, makeRefData(emp))

		// Matches the dashboard's ActiveEmployee gate: terminated → Not Rostered.
		assert.Equal(t, "not-rostered", timesheetMap[41].ReviewStatus)
	})

	t.Run("rostered ON, no assigned job → required", func(t *testing.T) {
		emp := makeEmp(4, 10) // active, on-cycle roster; JobID 0
		ts := model.OktediTimesheet{EmployeeID: 4, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{4: ts}

		updateReviewStatus(rosterOnDate, timesheetMap, makeRefData(emp))

		// No assigned job no longer gets special-cased — it's just a generic
		// "required" row that the reviewer resolves.
		assert.Equal(t, "required", timesheetMap[4].ReviewStatus)
	})

	t.Run("absent row → stays absent regardless of job", func(t *testing.T) {
		emp := makeEmp(10, 10) // active, on-cycle roster; JobID 0
		ts := model.OktediTimesheet{EmployeeID: 10, Hours: 0, ReviewStatus: "absent"}
		timesheetMap := map[int32]model.OktediTimesheet{10: ts}

		updateReviewStatus(rosterOnDate, timesheetMap, makeRefData(emp))

		// The absent guard runs after the roster classification but only for
		// on-cycle rows, so a genuine absent row is preserved.
		assert.Equal(t, "absent", timesheetMap[10].ReviewStatus)
	})

	t.Run("rostered ON, missing project → required", func(t *testing.T) {
		emp := makeEmp(5, 10) // active, on-cycle roster
		emp.JobID = 99
		ts := model.OktediTimesheet{EmployeeID: 5, Hours: 8, ProjectID: nil, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{5: ts}

		updateReviewStatus(rosterOnDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "required", timesheetMap[5].ReviewStatus)
	})

	t.Run("roster misconfigured (time type missing) → Missing Roster", func(t *testing.T) {
		emp := makeEmp(6, 99) // ttID 99 not in TimeTypeMap → timeType nil
		ts := model.OktediTimesheet{EmployeeID: 6, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{6: ts}

		updateReviewStatus(testDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "missing-roster", timesheetMap[6].ReviewStatus)
	})

	t.Run("roster misconfigured wins over not-rostered", func(t *testing.T) {
		// Start date set but no time type assigned → misconfigured, even though
		// IsRosteredOn would fail open to always-on.
		emp := models.Employee{EmployeeID: 7, RosterPayrollTimeTypeID: 0, RosterStartDate: rosterStart, EraID: int32(eraid.Present)}
		ts := model.OktediTimesheet{EmployeeID: 7, Hours: 8, ReviewStatus: "required"}
		timesheetMap := map[int32]model.OktediTimesheet{7: ts}

		updateReviewStatus(testDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "missing-roster", timesheetMap[7].ReviewStatus)
	})

	t.Run("adjusted matches rostered → auto-approved", func(t *testing.T) {
		emp := makeEmp(20, 10) // active, on-cycle roster
		emp.JobID = 7
		emp.CostCentreID = 3
		ts := model.OktediTimesheet{
			EmployeeID:   20,
			StartTime:    onCycleStart,
			Hours:        8, // break 0 → totalHours 8 == rostered span (08:00–16:00)
			ProjectID:    utils.Ptr(int32(7)),
			CostCentreID: utils.Ptr(int32(3)),
			ReviewStatus: "",
		}
		timesheetMap := map[int32]model.OktediTimesheet{20: ts}
		refData := makeRefData(emp)
		refData.EmpWorkHours = map[int32]map[int32]models.EmployeeWorkHour{
			20: {onCycleDow: {Start: "08:00", Finish: "16:00"}},
		}

		updateReviewStatus(rosterOnDate, timesheetMap, refData)

		assert.Equal(t, "", timesheetMap[20].ReviewStatus)
		assert.True(t, timesheetMap[20].Approved, "matching timesheet should be auto-approved")
	})

	t.Run("adjusted differs from rostered → required, not approved", func(t *testing.T) {
		emp := makeEmp(21, 10) // active, on-cycle roster
		emp.JobID = 7
		emp.CostCentreID = 3
		ts := model.OktediTimesheet{
			EmployeeID:   21,
			StartTime:    onCycleStart,
			Hours:        6, // != rostered span 8
			ProjectID:    utils.Ptr(int32(7)),
			CostCentreID: utils.Ptr(int32(3)),
			ReviewStatus: "",
		}
		timesheetMap := map[int32]model.OktediTimesheet{21: ts}
		refData := makeRefData(emp)
		refData.EmpWorkHours = map[int32]map[int32]models.EmployeeWorkHour{
			21: {onCycleDow: {Start: "08:00", Finish: "16:00"}},
		}

		updateReviewStatus(rosterOnDate, timesheetMap, refData)

		assert.Equal(t, "required", timesheetMap[21].ReviewStatus)
		assert.False(t, timesheetMap[21].Approved, "non-matching timesheet should not be auto-approved")
	})
}

func TestInjectAbsentRows(t *testing.T) {
	rosterStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	onRosterTT := models.PayrollTimeType{PayrollTimeTypeID: 10, RosteredDaysOn: 5, RosteredDaysOff: 2}
	offRosterTT := models.PayrollTimeType{PayrollTimeTypeID: 20, RosteredDaysOn: 5, RosteredDaysOff: 2}
	// Jan 10 is day 5 from Jan 5 → dayInCycle=5 → OFF for a 5/2 roster
	_ = offRosterTT

	t.Run("emp with clock-in already in map → no change", func(t *testing.T) {
		emp := models.Employee{EmployeeID: 1, RosterPayrollTimeTypeID: 10, RosterStartDate: rosterStart}
		existing := model.OktediTimesheet{EmployeeID: 1, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{1: existing}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		// Jan 10 is day 5 (rostered OFF for 5/2), but emp already has a record so no inject
		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		assert.Equal(t, 1, len(timesheetMap))
		assert.Equal(t, existing, timesheetMap[1])
	})

	t.Run("emp rostered ON, not in map → absent row created", func(t *testing.T) {
		// Use a roster start that makes Jan 10 an ON day: start Jan 6 → day 4 → ON
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		emp := models.Employee{EmployeeID: 2, RosterPayrollTimeTypeID: 10, RosterStartDate: start}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.Contains(t, timesheetMap, int32(2))
		row := timesheetMap[2]
		assert.Equal(t, float64(0), row.Hours)
		assert.Equal(t, "absent", row.ReviewStatus)
		assert.False(t, row.Approved)
	})

	t.Run("emp rostered OFF, not in map → no row created", func(t *testing.T) {
		// Jan 10 from Jan 5 = day 5 → OFF for 5/2 roster
		emp := models.Employee{EmployeeID: 3, RosterPayrollTimeTypeID: 10, RosterStartDate: rosterStart}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		assert.NotContains(t, timesheetMap, int32(3))
	})

	t.Run("emp with no roster (typeID=0) → skipped, no absent row", func(t *testing.T) {
		emp := models.Employee{EmployeeID: 4, RosterPayrollTimeTypeID: 0}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.NotContains(t, timesheetMap, int32(4))
	})

	// Termination/era filtering is now applied by the ActiveEmployee predicate
	// inside injectAbsentRows (it gates proactive absent rows only).

	t.Run("non-active era, rostered ON → no absent row", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC) // Jan 10 = ON
		emp := models.Employee{
			EmployeeID: 20, RosterPayrollTimeTypeID: 10, RosterStartDate: start,
			EraID: int32(eraid.Deleted),
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.NotContains(t, timesheetMap, int32(20))
	})

	t.Run("terminated before date, rostered ON → no absent row", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		emp := models.Employee{
			EmployeeID: 21, RosterPayrollTimeTypeID: 10, RosterStartDate: start,
			EraID: int32(eraid.Present), EndDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.NotContains(t, timesheetMap, int32(21))
	})

	t.Run("emp with JobID/CostCentreID → copied to absent row", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		emp := models.Employee{
			EmployeeID:              5,
			RosterPayrollTimeTypeID: 10,
			RosterStartDate:         start,
			JobID:                   10,
			CostCentreID:            20,
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.Contains(t, timesheetMap, int32(5))
		row := timesheetMap[5]
		require.NotNil(t, row.ProjectID)
		assert.Equal(t, utils.Ptr(int32(10)), row.ProjectID)
		require.NotNil(t, row.CostCentreID)
		assert.Equal(t, utils.Ptr(int32(20)), row.CostCentreID)
	})

	t.Run("supervisor filter set → only direct reports get absent rows", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		empUnderSup50 := models.Employee{
			EmployeeID: 6, RosterPayrollTimeTypeID: 10, RosterStartDate: start, ReportsToID: 50,
		}
		empUnderSup99 := models.Employee{
			EmployeeID: 7, RosterPayrollTimeTypeID: 10, RosterStartDate: start, ReportsToID: 99,
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData(
			[]models.Employee{empUnderSup50, empUnderSup99},
			map[int32]models.PayrollTimeType{10: onRosterTT},
		)

		opts := PrepareOptions{Supervisors: []int32{50}}
		injectAbsentRows(testDate, refData.Employees, opts, timesheetMap, refData)

		require.Contains(t, timesheetMap, int32(6), "emp under supervisor 50 should get absent row")
		require.NotContains(t, timesheetMap, int32(7), "emp under supervisor 99 should be skipped")
	})

	t.Run("roster misconfigured (time type missing) → Missing Roster row, not absent", func(t *testing.T) {
		// typeID 99 is assigned but not present in TimeTypeMap → timeType nil → misconfigured
		emp := models.Employee{EmployeeID: 11, RosterPayrollTimeTypeID: 99, RosterStartDate: rosterStart}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.Contains(t, timesheetMap, int32(11))
		row := timesheetMap[11]
		assert.Equal(t, "missing-roster", row.ReviewStatus)
		assert.Equal(t, float64(0), row.Hours)
	})
}

// A clock-in record must always be attributed to its employee, even one who is
// no longer active — the work happened and must be paid. Only proactive absent
// rows are gated by ActiveEmployee, not record attribution.
func TestProcessClockInRecordsAttributesNonActiveEmployee(t *testing.T) {
	emp := models.Employee{
		EmployeeID:        30,
		IdentificationTag: "TAG30",
		EraID:             int32(eraid.Present),
		EndDate:           time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), // terminated before testDate (Jan 10)
	}
	require.False(t, ActiveEmployee(emp, testDate), "precondition: emp is non-active on the date")

	records := []*model.ClockinRecord{
		{ID: "r1", Tag: "TAG30", Date: "2026-01-10", Timestamp: "2026-01-10T08:00:00Z"},
		{ID: "r2", Tag: "TAG30", Date: "2026-01-10", Timestamp: "2026-01-10T16:00:00Z"},
	}
	refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{})
	timesheetMap := map[int32]model.OktediTimesheet{}

	processed, errored := processClockInRecords(testDate, records, refData, timesheetMap)

	require.Contains(t, timesheetMap, int32(30), "worked hours must be recorded even for a terminated employee")
	assert.Empty(t, errored)
	assert.ElementsMatch(t, []string{"r1", "r2"}, processed)
}
