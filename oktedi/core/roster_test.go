package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/core/models"
	"github.com/stretchr/testify/assert"
)

func tt(daysOn, daysOff int32) *models.PayrollTimeType {
	return &models.PayrollTimeType{RosteredDaysOn: daysOn, RosteredDaysOff: daysOff}
}

func TestIsRosteredOn(t *testing.T) {
	startDate := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		emp      models.Employee
		timeType *models.PayrollTimeType
		date     time.Time
		expected bool
	}{
		{
			name:     "nil timeType → always on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: nil,
			date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "RosterPayrollTimeTypeID == 0 → always on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 0, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "RosterStartDate zero → always on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "daysOn=0 daysOff=0 → always on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(0, 0),
			date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "5-on/2-off, dayInCycle=0 → on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "5-on/2-off, dayInCycle=4 → on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "5-on/2-off, dayInCycle=5 → off",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "5-on/2-off, dayInCycle=6 → off",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "5-on/2-off, dayInCycle=7 (new cycle) → on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "date before RosterStartDate → on",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRosteredOn(tc.emp, tc.timeType, tc.date)
			assert.Equal(t, tc.expected, result)
		})
	}
}
