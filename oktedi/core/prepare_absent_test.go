package core

import (
	"testing"
	"time"

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
	for _, e := range employees {
		empMap[e.EmployeeID] = e
	}
	return &ReferenceData{
		Employees:       employees,
		EmpMap:          empMap,
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

	makeEmp := func(id int32, ttID int32) models.Employee {
		return models.Employee{EmployeeID: id, RosterPayrollTimeTypeID: ttID, RosterStartDate: rosterStart}
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

	t.Run("rostered ON, timesheet passes validation → empty status", func(t *testing.T) {
		emp := models.Employee{EmployeeID: 4} // no roster data at all → always ON
		emp.JobID = 0                         // no job requirement → passes
		ts := model.OktediTimesheet{EmployeeID: 4, Hours: 8, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{4: ts}
		refData := &ReferenceData{
			EmpMap:          map[int32]models.Employee{4: emp},
			TimeTypeMap:     map[int32]models.PayrollTimeType{},
			EmpWorkHours:    map[int32]map[int32]models.EmployeeWorkHour{},
			RegionWorkHours: map[int32]map[int32]models.RegionWorkHour{},
		}

		updateReviewStatus(testDate, timesheetMap, refData)

		// No project assigned → required (UpdateSingleReviewStatus behaviour)
		assert.Equal(t, "required", timesheetMap[4].ReviewStatus)
	})

	t.Run("rostered ON, missing project → required", func(t *testing.T) {
		emp := models.Employee{EmployeeID: 5} // no roster data at all → always ON
		emp.JobID = 99
		ts := model.OktediTimesheet{EmployeeID: 5, Hours: 8, ProjectID: nil, ReviewStatus: ""}
		timesheetMap := map[int32]model.OktediTimesheet{5: ts}
		refData := &ReferenceData{
			EmpMap:          map[int32]models.Employee{5: emp},
			TimeTypeMap:     map[int32]models.PayrollTimeType{},
			EmpWorkHours:    map[int32]map[int32]models.EmployeeWorkHour{},
			RegionWorkHours: map[int32]map[int32]models.RegionWorkHour{},
		}

		updateReviewStatus(testDate, timesheetMap, refData)

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
		emp := models.Employee{EmployeeID: 7, RosterPayrollTimeTypeID: 0, RosterStartDate: rosterStart}
		ts := model.OktediTimesheet{EmployeeID: 7, Hours: 8, ReviewStatus: "required"}
		timesheetMap := map[int32]model.OktediTimesheet{7: ts}

		updateReviewStatus(testDate, timesheetMap, makeRefData(emp))

		assert.Equal(t, "missing-roster", timesheetMap[7].ReviewStatus)
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

	t.Run("emp with EndDate older than date-7d → skipped", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		// testDate is 2026-01-10; threshold = 2026-01-03; emp ended 2026-01-02 → skip
		emp := models.Employee{
			EmployeeID:              8,
			RosterPayrollTimeTypeID: 10,
			RosterStartDate:         start,
			EndDate:                 time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.NotContains(t, timesheetMap, int32(8))
	})

	t.Run("emp with EndDate within 7-day grace → absent row created", func(t *testing.T) {
		start := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
		// testDate is 2026-01-10; threshold = 2026-01-03; emp ended 2026-01-08 → still inject
		emp := models.Employee{
			EmployeeID:              9,
			RosterPayrollTimeTypeID: 10,
			RosterStartDate:         start,
			EndDate:                 time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC),
		}
		timesheetMap := map[int32]model.OktediTimesheet{}
		refData := baseRefData([]models.Employee{emp}, map[int32]models.PayrollTimeType{10: onRosterTT})

		injectAbsentRows(testDate, refData.Employees, PrepareOptions{}, timesheetMap, refData)

		require.Contains(t, timesheetMap, int32(9))
		assert.Equal(t, "absent", timesheetMap[9].ReviewStatus)
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
