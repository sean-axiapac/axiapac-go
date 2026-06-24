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
			name:     "date before RosterStartDate → off (cycle not started)",
			emp:      models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType: tt(5, 2),
			date:     time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRosteredOn(tc.emp, tc.timeType, tc.date)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateRoster(t *testing.T) {
	startDate := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		emp          models.Employee
		timeType     *models.PayrollTimeType
		wantIsRoster bool
		wantValid    bool
		wantReason   string
	}{
		{
			name:         "no roster data at all → not a roster employee, valid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 0},
			timeType:     nil,
			wantIsRoster: false,
			wantValid:    true,
		},
		{
			name:         "start date set but no roster time type → roster, invalid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 0, RosterStartDate: startDate},
			timeType:     tt(5, 2),
			wantIsRoster: true,
			wantValid:    false,
			wantReason:   "roster start date set but roster payroll time type not set",
		},
		{
			name:         "time type set but no start date → roster, invalid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 1},
			timeType:     tt(5, 2),
			wantIsRoster: true,
			wantValid:    false,
			wantReason:   "roster payroll time type set but roster start date not set",
		},
		{
			name:         "roster assigned but timeType not found → roster, invalid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType:     nil,
			wantIsRoster: true,
			wantValid:    false,
			wantReason:   "roster time type not found",
		},
		{
			name:         "roster assigned but cycle 0/0 → roster, invalid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType:     tt(0, 0),
			wantIsRoster: true,
			wantValid:    false,
			wantReason:   "roster cycle (days on/off) not configured",
		},
		{
			name:         "roster fully configured → roster, valid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType:     tt(5, 2),
			wantIsRoster: true,
			wantValid:    true,
		},
		{
			name:         "roster with only days-on set → roster, valid",
			emp:          models.Employee{RosterPayrollTimeTypeID: 1, RosterStartDate: startDate},
			timeType:     tt(7, 0),
			wantIsRoster: true,
			wantValid:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isRoster, valid, reason := ValidateRoster(tc.emp, tc.timeType)
			assert.Equal(t, tc.wantIsRoster, isRoster)
			assert.Equal(t, tc.wantValid, valid)
			assert.Equal(t, tc.wantReason, reason)
		})
	}
}
