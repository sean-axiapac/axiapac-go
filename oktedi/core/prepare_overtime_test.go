package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"github.com/stretchr/testify/assert"
)

// applyOvertime runs after snapping, so ts.FinishTime is already the adjusted
// finish and ts.Hours is the full span (FinishTime - StartTime), before breaks.
func TestApplyOvertime(t *testing.T) {
	empID := int32(100)
	// Monday (1) defined hours 06:00 - 15:00.
	monday := time.Date(2023, 10, 23, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Monday, monday.Weekday())

	refData := &ReferenceData{
		EmpMap: map[int32]models.Employee{
			empID: {EmployeeID: empID, UseCalendarWorkHours: false},
		},
		EmpWorkHours: map[int32]map[int32]models.EmployeeWorkHour{
			empID: {1: {Start: "06:00", Finish: "15:00"}},
		},
	}

	newTS := func(start, finish time.Time) model.OktediTimesheet {
		return model.OktediTimesheet{
			EmployeeID: empID,
			StartTime:  start,
			FinishTime: finish,
			Hours:      finish.Sub(start).Hours(),
		}
	}

	tests := []struct {
		name             string
		start            time.Time
		finish           time.Time
		expectedOvertime float64
		expectedHours    float64
	}{
		{
			name:             "Finish beyond +15m tolerance -> overtime from defined finish",
			start:            time.Date(2023, 10, 23, 6, 0, 0, 0, time.UTC),
			finish:           time.Date(2023, 10, 23, 15, 30, 0, 0, time.UTC),
			expectedOvertime: 0.5, // 15:30 - 15:00
			expectedHours:    9.0, // 9.5 span - 0.5 overtime
		},
		{
			name:             "Finish exactly at defined finish -> no overtime",
			start:            time.Date(2023, 10, 23, 6, 0, 0, 0, time.UTC),
			finish:           time.Date(2023, 10, 23, 15, 0, 0, 0, time.UTC),
			expectedOvertime: 0.0,
			expectedHours:    9.0,
		},
		{
			name:             "Finish within +15m tolerance -> no overtime",
			start:            time.Date(2023, 10, 23, 6, 0, 0, 0, time.UTC),
			finish:           time.Date(2023, 10, 23, 15, 15, 0, 0, time.UTC),
			expectedOvertime: 0.0,
			expectedHours:    9.25,
		},
		{
			name:             "One hour of overtime",
			start:            time.Date(2023, 10, 23, 6, 0, 0, 0, time.UTC),
			finish:           time.Date(2023, 10, 23, 16, 0, 0, 0, time.UTC),
			expectedOvertime: 1.0,
			expectedHours:    9.0, // 10.0 span - 1.0 overtime
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsMap := map[int32]model.OktediTimesheet{empID: newTS(tt.start, tt.finish)}
			applyOvertime(tsMap, refData)
			got := tsMap[empID]
			assert.InDelta(t, tt.expectedOvertime, got.Overtime, 0.001, "overtime")
			assert.InDelta(t, tt.expectedHours, got.Hours, 0.001, "hours")
		})
	}
}

// Absent rows carry no clock times and must never accrue overtime.
func TestApplyOvertimeSkipsAbsent(t *testing.T) {
	empID := int32(100)
	refData := &ReferenceData{
		EmpMap: map[int32]models.Employee{
			empID: {EmployeeID: empID},
		},
		EmpWorkHours: map[int32]map[int32]models.EmployeeWorkHour{
			empID: {1: {Start: "06:00", Finish: "15:00"}},
		},
	}
	tsMap := map[int32]model.OktediTimesheet{
		empID: {EmployeeID: empID, ReviewStatus: "absent"},
	}
	applyOvertime(tsMap, refData)
	assert.Equal(t, 0.0, tsMap[empID].Overtime)
}
