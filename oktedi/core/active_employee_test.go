package core

import (
	"testing"
	"time"

	"axiapac.com/axiapac/axiapac/v1/common/eraid"
	"axiapac.com/axiapac/core/models"
	"github.com/stretchr/testify/assert"
)

// asOf is the date the dashboard/prepare flow is targeting.
var activeAsOf = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

func TestActiveEmployee(t *testing.T) {
	d := func(y int, m time.Month, day, hour int) time.Time {
		return time.Date(y, m, day, hour, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name    string
		era     int32
		endDate time.Time
		want    bool
	}{
		{"present era, no end date (NULL)", int32(eraid.Present), time.Time{}, true},
		{"present era, end date == 1900-01-01 sentinel", int32(eraid.Present), d(1900, 1, 1, 0), true},
		{"present era, future termination", int32(eraid.Present), d(2026, 2, 1, 0), true},
		{"present era, terminated exactly on asOf", int32(eraid.Present), d(2026, 1, 10, 0), true},
		{"present era, terminated later same day (time-of-day ignored)", int32(eraid.Present), d(2026, 1, 10, 15), true},
		{"present era, terminated day before asOf", int32(eraid.Present), d(2026, 1, 9, 0), false},
		{"present era, terminated well before asOf", int32(eraid.Present), d(2026, 1, 5, 0), false},
		{"draft era, otherwise active", int32(eraid.Draft), time.Time{}, false},
		{"deleted era, otherwise active", int32(eraid.Deleted), time.Time{}, false},
		{"archived era, future termination", int32(eraid.Archived), d(2026, 2, 1, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emp := models.Employee{EmployeeID: 1, EraID: tt.era, EndDate: tt.endDate}
			assert.Equal(t, tt.want, ActiveEmployee(emp, activeAsOf))
		})
	}
}
