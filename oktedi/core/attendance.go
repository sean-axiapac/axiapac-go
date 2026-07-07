package core

import (
	"strings"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"gorm.io/gorm"
)

// AttendanceRow is one employee's attendance state for a single date. It is the
// read-only, dashboard-facing counterpart to the timesheet prepare flow: instead
// of writing timesheets it classifies who worked / who is missing.
//
// The dashboard cards are derived on the client from three facts —
// Rostered, RecordCount and whether the viewed date is today — so this row stays
// intentionally card-agnostic. Absent-streak fields are populated only for
// rostered employees with no records on the date (i.e. the Absent card).
type AttendanceRow struct {
	EmployeeID  int32  `json:"employeeId"`
	Code        string `json:"code"`
	FirstName   string `json:"firstName"`
	Surname     string `json:"surname"`
	Rostered    bool   `json:"rostered"`    // scheduled to work on the viewed date
	RecordCount int    `json:"recordCount"` // clock records on the viewed date
	// Clock times in Brisbane "HH:MM". ClockOn is the earliest record; ClockOff
	// the latest, and is nil unless there are at least two records.
	ClockOn  *string `json:"clockOn"`
	ClockOff *string `json:"clockOff"`
	// Scheduled break in minutes from the roster work-hours (nil when the day
	// has no defined work hours). Not deducted from Total Hours.
	BreakMinutes *int32 `json:"breakMinutes"`
	// Project resolved from the employee's assigned job (roster/home), so absent
	// employees still carry a project for the header filter.
	ProjectID   int32  `json:"projectId"`
	ProjectCode string `json:"projectCode"`
	ProjectName string `json:"projectName"`
	// ReviewStatus is the prepared timesheet's review state for the date
	// (e.g. "required", "absent", "accurate"), or "" when no timesheet exists
	// yet (typically today, before the prepare flow runs). Raw lowercase; the
	// client formats it for display.
	ReviewStatus string `json:"reviewStatus"`
	// Absent-only streaks (nil when the employee is not absent on the date).
	ConsecutiveDaysAbsent *int `json:"consecutiveDaysAbsent"`
	TotalAbsentDays       *int `json:"totalAbsentDays"`
	// Panel: `rosterPanel` from the employee's Attributes JSON ("" when absent).
	Panel string `json:"panel"`
	// DeviceID is the device of the clock-in record ("" when the employee has no
	// records on the date). The evacuation register maps it to a physical area.
	DeviceID string `json:"deviceId"`
	// Evacuation-register fields. Employer resolves the Attributes `employer.id`
	// reference to the supplier name; Department joins the Attributes
	// `businessUnit` and `department` values (e.g. "Projects BU FIFO Village");
	// Area is the Attributes `area` value (not yet populated in current data);
	// Classification is the employee's Occupation description.
	Employer       string `json:"employer"`
	Department     string `json:"department"`
	Area           string `json:"area"`
	Classification string `json:"classification"`
}

// AttendanceResult is the full payload for the dashboard: the per-employee rows
// plus the flags the client needs to pick card behaviour.
type AttendanceResult struct {
	Date    string          `json:"date"`
	IsToday bool            `json:"isToday"`
	Rows    []AttendanceRow `json:"rows"`
}

// AbsentLookbackDays caps how far back the absent-streak walks go, for
// performance and to avoid unbounded history reads.
const AbsentLookbackDays = 90

// CountConsecutiveAbsent counts the unbroken run of *scheduled* days with no
// record, walking backwards from `from` (inclusive). Rostered-off days don't
// break the streak — only a scheduled day that has a record does. Capped at
// lookbackDays. hasRecord reports whether the employee clocked on that date.
func CountConsecutiveAbsent(emp models.Employee, timeType *models.PayrollTimeType, from time.Time, hasRecord func(time.Time) bool, lookbackDays int) int {
	count := 0
	for i := 0; i <= lookbackDays; i++ {
		d := from.AddDate(0, 0, -i)
		if !IsRosteredOn(emp, timeType, d) {
			continue // not scheduled — doesn't count, doesn't break the run
		}
		if hasRecord(d) {
			break // scheduled and present — the streak ends here
		}
		count++
	}
	return count
}

// CountTotalAbsent counts all scheduled days with no record from `today` back
// over lookbackDays (inclusive). Unlike the consecutive count this is a fixed
// snapshot to today and does not stop at the first attended day.
func CountTotalAbsent(emp models.Employee, timeType *models.PayrollTimeType, today time.Time, hasRecord func(time.Time) bool, lookbackDays int) int {
	count := 0
	for i := 0; i <= lookbackDays; i++ {
		d := today.AddDate(0, 0, -i)
		if !IsRosteredOn(emp, timeType, d) {
			continue
		}
		if !hasRecord(d) {
			count++
		}
	}
	return count
}

// attendanceRefData is the lean reference set the dashboard needs. It
// deliberately omits cost-centre / job-cost-centre data that the prepare flow
// loads, since the dashboard only needs project name resolution.
type attendanceRefData struct {
	employees       []models.Employee
	timeTypeMap     map[int32]models.PayrollTimeType
	jobByID         map[int32]models.Job
	empWorkHours    map[int32]map[int32]models.EmployeeWorkHour
	regionWorkHours map[int32]map[int32]models.RegionWorkHour
	supplierNames   map[int32]string // employer resolution (Attributes employer.id)
	occupationDescs map[int32]string // classification (Employees.OccupationId)
}

func loadAttendanceRefData(db *gorm.DB) (*attendanceRefData, error) {
	// Load ALL employees so clock-in records for anyone (active or not) still map
	// to an employee. Active-only gating happens in the roster pass below.
	var employees []models.Employee
	if err := db.Find(&employees).Error; err != nil {
		return nil, err
	}
	var timeTypes []models.PayrollTimeType
	if err := db.Find(&timeTypes).Error; err != nil {
		return nil, err
	}
	ttMap := make(map[int32]models.PayrollTimeType, len(timeTypes))
	for _, tt := range timeTypes {
		ttMap[tt.PayrollTimeTypeID] = tt
	}
	var jobs []models.Job
	if err := db.Find(&jobs).Error; err != nil {
		return nil, err
	}
	jobByID := make(map[int32]models.Job, len(jobs))
	for _, j := range jobs {
		jobByID[j.JobID] = j
	}

	// Work hours (for the scheduled break minutes), same source the prepare
	// flow uses via GetBreakMinutes.
	var empWH []models.EmployeeWorkHour
	if err := db.Find(&empWH).Error; err != nil {
		return nil, err
	}
	empWHMap := make(map[int32]map[int32]models.EmployeeWorkHour)
	for _, wh := range empWH {
		if empWHMap[wh.EmployeeID] == nil {
			empWHMap[wh.EmployeeID] = make(map[int32]models.EmployeeWorkHour)
		}
		empWHMap[wh.EmployeeID][wh.DayOfWeek] = wh
	}
	var regionWH []models.RegionWorkHour
	if err := db.Find(&regionWH).Error; err != nil {
		return nil, err
	}
	regionWHMap := make(map[int32]map[int32]models.RegionWorkHour)
	for _, wh := range regionWH {
		if regionWHMap[wh.CalendarRegionID] == nil {
			regionWHMap[wh.CalendarRegionID] = make(map[int32]models.RegionWorkHour)
		}
		regionWHMap[wh.CalendarRegionID][wh.DayOfWeek] = wh
	}

	// Suppliers (employer names) and occupations (classification) for the
	// evacuation-register fields.
	var suppliers []models.Supplier
	if err := db.Find(&suppliers).Error; err != nil {
		return nil, err
	}
	supplierNames := make(map[int32]string, len(suppliers))
	for _, s := range suppliers {
		supplierNames[s.SupplierID] = s.Name
	}
	var occupations []models.Occupation
	if err := db.Find(&occupations).Error; err != nil {
		return nil, err
	}
	occupationDescs := make(map[int32]string, len(occupations))
	for _, o := range occupations {
		occupationDescs[o.OccupationID] = o.Description
	}

	return &attendanceRefData{
		employees:       employees,
		timeTypeMap:     ttMap,
		jobByID:         jobByID,
		empWorkHours:    empWHMap,
		regionWorkHours: regionWHMap,
		supplierNames:   supplierNames,
		occupationDescs: occupationDescs,
	}, nil
}

// timeTypeFor resolves an employee's roster time type (nil when unset/unknown).
func (rd *attendanceRefData) timeTypeFor(emp models.Employee) *models.PayrollTimeType {
	if emp.RosterPayrollTimeTypeID == 0 {
		return nil
	}
	if tt, ok := rd.timeTypeMap[emp.RosterPayrollTimeTypeID]; ok {
		return &tt
	}
	return nil
}

func formatBrisbaneClock(ts string) *string {
	if ts == "" {
		return nil
	}
	t, err := utils.ParseISOTime(ts)
	if err != nil {
		return nil
	}
	bt := utils.AdjustUtcToBrisbaneHours(t)
	s := bt.Format("15:04")
	return &s
}

// LoadAttendance builds the attendance view for `date`. It returns every roster
// employee scheduled on that date (worked or absent) plus anyone who clocked in
// without being scheduled (the Not Rostered case). `now` supplies "today" in
// Brisbane for the isToday flag and the total-absent snapshot.
func LoadAttendance(db *gorm.DB, date time.Time, now time.Time) (*AttendanceResult, error) {
	date = truncateDay(date)
	today := truncateDay(now)
	dateStr := date.Format("2006-01-02")

	refData, err := loadAttendanceRefData(db)
	if err != nil {
		return nil, err
	}

	// Clock records for the viewed date, grouped per employee (by tag).
	var dayRecords []*model.ClockinRecord
	if err := db.Where("date = ?", dateStr).Find(&dayRecords).Error; err != nil {
		return nil, err
	}
	groupByTag := make(map[string]*RecordGroup)
	for _, g := range GroupRecords(dayRecords) {
		groupByTag[g.Tag] = g
	}

	// Prepared timesheets for the date, for the Review Status column. Keyed by
	// employee; absent when the prepare flow hasn't run yet (leaves status "").
	var dayTimesheets []model.OktediTimesheet
	if err := db.Where("date = ?", dateStr).Find(&dayTimesheets).Error; err != nil {
		return nil, err
	}
	reviewByEmp := make(map[int32]string, len(dayTimesheets))
	for _, ts := range dayTimesheets {
		reviewByEmp[ts.EmployeeID] = ts.ReviewStatus
	}

	rows := make([]AttendanceRow, 0, len(refData.employees))
	seenTags := make(map[string]bool)

	// Pass 1: roster employees scheduled on the date (worked or absent), and
	// roster/non-roster employees who clocked in.
	var absentEmps []models.Employee
	for _, emp := range refData.employees {
		group := groupByTag[emp.IdentificationTag]
		hasRecords := group != nil && emp.IdentificationTag != ""
		if hasRecords {
			seenTags[emp.IdentificationTag] = true
		}

		timeType := refData.timeTypeFor(emp)
		active := ActiveEmployee(emp, date)
		class := ClassifyRoster(emp, timeType, date)
		rosteredOn := class == RosterOnCycle

		// Show worked employees always. For no clock-in, show only active staff
		// who are rostered-on (→ Absent) or roster-misconfigured (→ Not Rostered),
		// mirroring the daily review. No-show non-roster/off-cycle/terminated
		// employees are ignored (they aren't prepared as timesheets either).
		if !hasRecords && !(active && (class == RosterOnCycle || class == RosterMissing)) {
			continue
		}

		row := buildRow(emp, refData)
		row.Rostered = rosteredOn
		row.ReviewStatus = reviewByEmp[emp.EmployeeID]
		row.BreakMinutes = GetBreakMinutes(date, emp, refData.empWorkHours, refData.regionWorkHours)
		if hasRecords {
			row.RecordCount = len(group.Records)
			row.ClockOn = formatBrisbaneClock(group.GetClockIn())
			row.DeviceID = group.GetDeviceID()
			if row.RecordCount >= 2 {
				row.ClockOff = formatBrisbaneClock(group.GetClockOut())
			}
		}

		if rosteredOn && !hasRecords {
			absentEmps = append(absentEmps, emp) // streaks computed in pass 2
		}
		rows = append(rows, row)
	}

	// Pass 2: absent-streak fields. Fetch records once over the lookback window
	// for the absent employees' tags, then count per employee.
	if len(absentEmps) > 0 {
		if err := enrichAbsentStreaks(db, refData, absentEmps, rows, date, today); err != nil {
			return nil, err
		}
	}

	return &AttendanceResult{Date: dateStr, IsToday: date.Equal(today), Rows: rows}, nil
}

func buildRow(emp models.Employee, refData *attendanceRefData) AttendanceRow {
	attrs := ParseAttributes(emp)
	row := AttendanceRow{
		EmployeeID:     emp.EmployeeID,
		Code:           emp.Code,
		FirstName:      emp.FirstName,
		Surname:        emp.Surname,
		ProjectID:      emp.JobID,
		Panel:          AttrString(attrs, "rosterPanel"),
		Employer:       refData.supplierNames[AttrRefID(attrs, "employer")],
		Department:     strings.TrimSpace(AttrString(attrs, "businessUnit") + " " + AttrString(attrs, "department")),
		Area:           AttrString(attrs, "area"),
		Classification: refData.occupationDescs[emp.OccupationID],
	}
	if job, ok := refData.jobByID[emp.JobID]; ok {
		row.ProjectCode = job.JobNo
		row.ProjectName = job.Description
	}
	return row
}

// enrichAbsentStreaks fills ConsecutiveDaysAbsent / TotalAbsentDays for the
// absent rows, reading the lookback window's records in one query.
func enrichAbsentStreaks(db *gorm.DB, refData *attendanceRefData, absentEmps []models.Employee, rows []AttendanceRow, date, today time.Time) error {
	tags := make([]string, 0, len(absentEmps))
	for _, e := range absentEmps {
		if e.IdentificationTag != "" {
			tags = append(tags, e.IdentificationTag)
		}
	}

	// Window spans the earlier of (date, today) minus the cap, to the later of
	// the two — covering both the consecutive walk (back from date) and the
	// total snapshot (back from today).
	windowStart := minTime(date, today).AddDate(0, 0, -AbsentLookbackDays)
	windowEnd := maxTime(date, today)
	present := make(map[string]bool) // "tag|YYYY-MM-DD"
	if len(tags) > 0 {
		var windowRecords []*model.ClockinRecord
		if err := db.Where("tag IN ? AND date >= ? AND date <= ?", tags,
			windowStart.Format("2006-01-02"), windowEnd.Format("2006-01-02")).
			Find(&windowRecords).Error; err != nil {
			return err
		}
		for _, r := range windowRecords {
			present[r.Tag+"|"+r.Date] = true
		}
	}

	empByID := make(map[int32]models.Employee, len(absentEmps))
	for _, e := range absentEmps {
		empByID[e.EmployeeID] = e
	}

	for i := range rows {
		emp, ok := empByID[rows[i].EmployeeID]
		if !ok {
			continue // not an absent row
		}
		tt := refData.timeTypeFor(emp)
		hasRecord := func(d time.Time) bool {
			if emp.IdentificationTag == "" {
				return false
			}
			return present[emp.IdentificationTag+"|"+d.Format("2006-01-02")]
		}
		consecutive := CountConsecutiveAbsent(emp, tt, date, hasRecord, AbsentLookbackDays)
		total := CountTotalAbsent(emp, tt, today, hasRecord, AbsentLookbackDays)
		rows[i].ConsecutiveDaysAbsent = &consecutive
		rows[i].TotalAbsentDays = &total
	}
	return nil
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
