package timesheet

import (
	"time"

	"axiapac.com/axiapac/core/models"
	oktedi "axiapac.com/axiapac/oktedi/core"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"gorm.io/gorm"
)

// enrichReviewColumns populates the daily-review-only derived fields on results:
//   - RosteredHours: the employee's assigned work-hours duration for the day
//     (same source as the snapping rules).
//   - ClockOn/ClockOff/Worked: the raw min/max clock times for the employee+date
//     from oktedi_records (Brisbane time), matching how prepare derives them.
//
// These are not stored on the timesheet, so they're computed on read. The work
// is batched (a few IN-queries scoped to the page) and reuses the core helpers
// so the values stay consistent with timesheet preparation. Enrichment is
// best-effort: a failed sub-query leaves the derived columns blank rather than
// failing the already-loaded list.
func enrichReviewColumns(db *gorm.DB, results []OktediTimesheetDTO) {
	if len(results) == 0 {
		return
	}

	empIDs := make([]int32, 0, len(results))
	dateStrs := make([]string, 0, len(results))
	seenEmp := map[int32]bool{}
	seenDate := map[string]bool{}
	for _, r := range results {
		if r.Employee.ID != 0 && !seenEmp[r.Employee.ID] {
			seenEmp[r.Employee.ID] = true
			empIDs = append(empIDs, r.Employee.ID)
		}
		ds := r.Date.Format("2006-01-02")
		if !seenDate[ds] {
			seenDate[ds] = true
			dateStrs = append(dateStrs, ds)
		}
	}

	// Employees (tag + work-hours config).
	var emps []models.Employee
	db.Where("EmployeeId IN ?", empIDs).Find(&emps)
	empByID := make(map[int32]models.Employee, len(emps))
	tags := make([]string, 0, len(emps))
	regionIDs := make([]int32, 0)
	for _, e := range emps {
		empByID[e.EmployeeID] = e
		if e.IdentificationTag != "" {
			tags = append(tags, e.IdentificationTag)
		}
		if e.CalendarRegionID != 0 {
			regionIDs = append(regionIDs, e.CalendarRegionID)
		}
	}

	// Work-hours maps keyed by day-of-week.
	empWH := make(map[int32]map[int32]models.EmployeeWorkHour)
	var ewh []models.EmployeeWorkHour
	db.Where("EmployeeId IN ?", empIDs).Find(&ewh)
	for _, wh := range ewh {
		if empWH[wh.EmployeeID] == nil {
			empWH[wh.EmployeeID] = make(map[int32]models.EmployeeWorkHour)
		}
		empWH[wh.EmployeeID][wh.DayOfWeek] = wh
	}
	regionWH := make(map[int32]map[int32]models.RegionWorkHour)
	if len(regionIDs) > 0 {
		var rwh []models.RegionWorkHour
		db.Where("CalendarRegionId IN ?", regionIDs).Find(&rwh)
		for _, wh := range rwh {
			if regionWH[wh.CalendarRegionID] == nil {
				regionWH[wh.CalendarRegionID] = make(map[int32]models.RegionWorkHour)
			}
			regionWH[wh.CalendarRegionID][wh.DayOfWeek] = wh
		}
	}

	// Raw clock pairs keyed by "tag|date" (Brisbane-adjusted).
	type clockPair struct{ in, out *time.Time }
	clockByTagDate := make(map[string]clockPair)
	if len(tags) > 0 {
		var records []*model.ClockinRecord
		db.Where("tag IN ? AND date IN ?", tags, dateStrs).Find(&records)
		byDate := make(map[string][]*model.ClockinRecord)
		for _, rec := range records {
			byDate[rec.Date] = append(byDate[rec.Date], rec)
		}
		for ds, recs := range byDate {
			for _, g := range oktedi.GroupRecords(recs) {
				var pair clockPair
				if s := g.GetClockIn(); s != "" {
					if t, err := utils.ParseISOTime(s); err == nil {
						pair.in = utils.AdjustUtcToBrisbaneHours(t)
					}
				}
				if s := g.GetClockOut(); s != "" {
					if t, err := utils.ParseISOTime(s); err == nil {
						pair.out = utils.AdjustUtcToBrisbaneHours(t)
					}
				}
				clockByTagDate[g.Tag+"|"+ds] = pair
			}
		}
	}

	for i := range results {
		r := &results[i]
		emp, ok := empByID[r.Employee.ID]
		if !ok {
			continue
		}

		// Rostered hours = assigned work-hours span for the day.
		if def, found := oktedi.GetDefinedWorkHours(r.Date, emp, empWH, regionWH); found {
			base := time.Date(r.Date.Year(), r.Date.Month(), r.Date.Day(), 0, 0, 0, 0, r.Date.Location())
			start, e1 := oktedi.ParseTimeOnDate(base, def.Start)
			finish, e2 := oktedi.ParseTimeOnDate(base, def.Finish)
			if e1 == nil && e2 == nil {
				if finish.Before(start) {
					finish = finish.Add(24 * time.Hour)
				}
				h := finish.Sub(start).Hours()
				r.RosteredHours = &h
				ss := start.Format("15:04")
				fs := finish.Format("15:04")
				r.RosteredStart = &ss
				r.RosteredFinish = &fs
			}
		}

		// Raw clock on/off/worked.
		if pair, ok := clockByTagDate[emp.IdentificationTag+"|"+r.Date.Format("2006-01-02")]; ok {
			if pair.in != nil {
				s := pair.in.Format("2006-01-02T15:04:05")
				r.ClockOn = &s
			}
			if pair.out != nil {
				s := pair.out.Format("2006-01-02T15:04:05")
				r.ClockOff = &s
			}
			if pair.in != nil && pair.out != nil {
				// Worked = clock off - clock on - break, floored at 0.
				w := pair.out.Sub(*pair.in).Hours()
				if r.Break != nil {
					w -= float64(*r.Break) / 60.0
				}
				if w < 0 {
					w = 0
				}
				r.Worked = &w
			}
		}
	}
}
