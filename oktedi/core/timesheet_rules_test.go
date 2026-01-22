package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/core/models"
	"github.com/stretchr/testify/assert"
)

func TestApplyStartRule(t *testing.T) {
	defined := time.Date(2023, 1, 1, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		actual   time.Time
		expected time.Time
	}{
		{
			name:     "Exact match",
			actual:   defined,
			expected: defined,
		},
		{
			name:     "Early within threshold (15m)",
			actual:   defined.Add(-15 * time.Minute),
			expected: defined,
		},
		{
			name:     "Early within threshold (10m)",
			actual:   defined.Add(-10 * time.Minute),
			expected: defined,
		},
		{
			name:     "Late within threshold (10m)",
			actual:   defined.Add(10 * time.Minute),
			expected: defined,
		},
		{
			name:     "Late within threshold (5m)",
			actual:   defined.Add(5 * time.Minute),
			expected: defined,
		},
		{
			name:     "Early outside threshold (16m)",
			actual:   defined.Add(-16 * time.Minute),
			expected: defined.Add(-16 * time.Minute),
		},
		{
			name:     "Late outside threshold (11m)",
			actual:   defined.Add(11 * time.Minute),
			expected: defined.Add(11 * time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ApplyStartRule(tt.actual, defined)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestApplyFinishRule(t *testing.T) {
	defined := time.Date(2023, 1, 1, 17, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		actual   time.Time
		expected time.Time
	}{
		{
			name:     "Exact match",
			actual:   defined,
			expected: defined,
		},
		{
			name:     "Early within threshold (10m)",
			actual:   defined.Add(-10 * time.Minute),
			expected: defined,
		},
		{
			name:     "Late within threshold (15m)",
			actual:   defined.Add(15 * time.Minute),
			expected: defined,
		},
		{
			name:     "Early outside threshold (11m)",
			actual:   defined.Add(-11 * time.Minute),
			expected: defined.Add(-11 * time.Minute),
		},
		{
			name:     "Late outside threshold (16m)",
			actual:   defined.Add(16 * time.Minute),
			expected: defined.Add(16 * time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ApplyFinishRule(tt.actual, defined)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestAdjustTimesheetHours(t *testing.T) {
	// Setup mock data
	empID := int32(100)
	regionID := int32(5)

	// Employee using User hours
	empUser := models.Employee{
		EmployeeID:           empID,
		UseCalendarWorkHours: false,
	}

	// Employee using Region hours
	empRegion := models.Employee{
		EmployeeID:           empID,
		UseCalendarWorkHours: true,
		CalendarRegionID:     regionID,
	}

	// Work hours data
	// Monday (1) 08:00 - 16:00
	empHours := map[int32]map[int32]models.EmployeeWorkHour{
		empID: {
			1: {Start: "08:00", Finish: "16:00"},
		},
	}

	// Region hours
	// Monday (1) 09:00 - 17:00
	regionHours := map[int32]map[int32]models.RegionWorkHour{
		regionID: {
			1: {Start: "09:00", Finish: "17:00"},
		},
	}

	// Test Date: Monday

	t.Run("User Hours - Within Threshold", func(t *testing.T) {
		actualStart := time.Date(2023, 10, 23, 7, 50, 0, 0, time.UTC)   // 07:50 (10m early for 08:00)
		actualFinish := time.Date(2023, 10, 23, 16, 10, 0, 0, time.UTC) // 16:10 (10m late for 16:00)

		res, err := AdjustTimesheetHours(actualStart, actualFinish, empUser, empHours, regionHours)
		assert.NoError(t, err)

		expectedStart := time.Date(2023, 10, 23, 8, 0, 0, 0, time.UTC)
		expectedFinish := time.Date(2023, 10, 23, 16, 0, 0, 0, time.UTC)

		assert.Equal(t, expectedStart, res.StartTime)
		assert.Equal(t, expectedFinish, res.FinishTime)
	})

	t.Run("Region Hours - Outside Threshold", func(t *testing.T) {
		// Region Start 09:00. Arrive 08:00 (60m early) -> Use Actual 08:00
		actualStart := time.Date(2023, 10, 23, 8, 0, 0, 0, time.UTC)
		// Region Finish 17:00. Leave 17:05 (5m late) -> Use Defined 17:00
		actualFinish := time.Date(2023, 10, 23, 17, 5, 0, 0, time.UTC)

		res, err := AdjustTimesheetHours(actualStart, actualFinish, empRegion, empHours, regionHours)
		assert.NoError(t, err)

		expectedStart := actualStart                                     // 08:00 (outside 15m early threshold)
		expectedFinish := time.Date(2023, 10, 23, 17, 0, 0, 0, time.UTC) // Snap to 17:00

		assert.Equal(t, expectedStart, res.StartTime)
		assert.Equal(t, expectedFinish, res.FinishTime)
	})

	t.Run("No Defined Hours", func(t *testing.T) {
		// Sunday (0) - no hours defined
		actualStart := time.Date(2023, 10, 22, 10, 0, 0, 0, time.UTC)
		actualFinish := time.Date(2023, 10, 22, 12, 0, 0, 0, time.UTC)

		// Note: AdjustTimesheetHours derives date from actualStart.
		res, err := AdjustTimesheetHours(actualStart, actualFinish, empUser, empHours, regionHours)
		assert.NoError(t, err)

		assert.Equal(t, actualStart, res.StartTime)
		assert.Equal(t, actualFinish, res.FinishTime)
	})
}
