package core

import (
	"fmt"
	"math"
	"sort"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"gorm.io/gorm"
)

type PrepareOptions struct {
	StartDate   time.Time
	EndDate     time.Time
	Supervisors []int32
	Employees   []int32
}

// PrepareSummary reports what a Prepare run did to the timesheet rows, so the
// caller can reassure the user that approved work was preserved.
type PrepareSummary struct {
	New          int `json:"new"`          // rows created (no prior row existed)
	Recomputed   int `json:"recomputed"`   // existing unapproved rows refreshed from the clock
	KeptApproved int `json:"keptApproved"` // existing approved rows left untouched
	KeptAbsent   int `json:"keptAbsent"`   // existing absent rows preserved
}

func Prepare(db *gorm.DB, opts PrepareOptions) (PrepareSummary, error) {
	var summary PrepareSummary
	// iterate through each day in the range
	for d := opts.StartDate; !d.After(opts.EndDate); d = d.AddDate(0, 0, 1) {
		if err := ProcessClockInRecordsWithFilters(db, d, opts, &summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

type ReferenceData struct {
	Employees       []models.Employee
	EmpMap          map[int32]models.Employee
	TagMap          map[string]models.Employee
	JobMap          map[string]models.Job
	JobCCMap        map[int32]map[string]models.CostCentre
	EmpWorkHours    map[int32]map[int32]models.EmployeeWorkHour
	RegionWorkHours map[int32]map[int32]models.RegionWorkHour
	TimeTypeMap     map[int32]models.PayrollTimeType
}

func ProcessClockInRecordsWithFilters(db *gorm.DB, date time.Time, opts PrepareOptions, summary *PrepareSummary) error {
	dateStr := date.Format("2006-01-02")

	// 1. Fetch Reference Data
	refData, err := fetchReferenceData(db)
	if err != nil {
		return err
	}

	// 2. Fetch Records
	supervisorRecords, clockInRecords, err := fetchRecords(db, dateStr, opts, refData.Employees)
	if err != nil {
		return err
	}

	// 3. Process Records
	// Map EmployeeID -> OktediTimesheet
	timesheetMap := make(map[int32]model.OktediTimesheet)

	// Step 1: Prepare from Clock-in Records (focus on start/finish)
	processedClockInIDs, errorClockInIDs := processClockInRecords(date, clockInRecords, refData, timesheetMap)

	// Step 2: Apply Supervisor Overrides (starttime, finishtime, and job/costcentres)
	applySupervisorRecords(date, supervisorRecords, timesheetMap, refData)

	// Step 2.5: Inject absent rows for rostered-on employees with no record
	injectAbsentRows(date, refData.Employees, opts, timesheetMap, refData)

	// Step 2.6: Remove seconds from start and finish times
	removeSeconds(timesheetMap)

	// Step 3: Apply Snapping rules based on defined work hours
	applySnappingRules(timesheetMap, refData)

	// Step 4: Deduct break time if applicable
	applyBreaks(timesheetMap)

	// Update review status based on final hours matching
	updateReviewStatus(date, timesheetMap, refData)

	// 4. Persist to DB
	if err := persistTimesheets(db, dateStr, timesheetMap, summary); err != nil {
		return err
	}

	// 5. Update Status of ClockIn Records
	updateProcessStatuses(db, processedClockInIDs, nil, errorClockInIDs)

	fmt.Println("Done.")
	return nil
}

func fetchReferenceData(db *gorm.DB) (*ReferenceData, error) {
	fmt.Println("Fetching reference data...")
	var employees []models.Employee
	if err := db.Find(&employees).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch employees: %w", err)
	}
	empMap := make(map[int32]models.Employee)
	tagMap := make(map[string]models.Employee)
	for _, e := range employees {
		empMap[e.EmployeeID] = e
		if e.IdentificationTag != "" {
			tagMap[e.IdentificationTag] = e
		}
	}

	var jobs []models.Job
	if err := db.Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch jobs: %w", err)
	}
	jobMap := make(map[string]models.Job)
	for _, j := range jobs {
		jobMap[j.JobNo] = j
	}

	var allCC []models.CostCentre
	if err := db.Find(&allCC).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch cost centres: %w", err)
	}
	ccByID := make(map[int32]models.CostCentre)
	for _, cc := range allCC {
		ccByID[cc.CostCentreID] = cc
	}

	var jobCCs []models.JobCostCentre
	if err := db.Find(&jobCCs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch job cost centres: %w", err)
	}

	jobCCMap := make(map[int32]map[string]models.CostCentre)
	for _, jcc := range jobCCs {
		cc, ok := ccByID[jcc.CostCentreID]
		if !ok {
			continue
		}
		if _, ok := jobCCMap[jcc.JobID]; !ok {
			jobCCMap[jcc.JobID] = make(map[string]models.CostCentre)
		}
		jobCCMap[jcc.JobID][cc.Code] = cc
	}

	// Fetch Employee Work Hours
	var empWorkHours []models.EmployeeWorkHour
	if err := db.Find(&empWorkHours).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch employee work hours: %w", err)
	}
	empWHMap := make(map[int32]map[int32]models.EmployeeWorkHour)
	for _, wh := range empWorkHours {
		if _, ok := empWHMap[wh.EmployeeID]; !ok {
			empWHMap[wh.EmployeeID] = make(map[int32]models.EmployeeWorkHour)
		}
		empWHMap[wh.EmployeeID][wh.DayOfWeek] = wh
	}

	// Fetch Region Work Hours
	var regionWorkHours []models.RegionWorkHour
	if err := db.Find(&regionWorkHours).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch region work hours: %w", err)
	}
	regionWHMap := make(map[int32]map[int32]models.RegionWorkHour)
	for _, wh := range regionWorkHours {
		if _, ok := regionWHMap[wh.CalendarRegionID]; !ok {
			regionWHMap[wh.CalendarRegionID] = make(map[int32]models.RegionWorkHour)
		}
		regionWHMap[wh.CalendarRegionID][wh.DayOfWeek] = wh
	}

	var timeTypes []models.PayrollTimeType
	if err := db.Find(&timeTypes).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch payroll time types: %w", err)
	}
	ttMap := make(map[int32]models.PayrollTimeType)
	for _, tt := range timeTypes {
		ttMap[tt.PayrollTimeTypeID] = tt
	}

	return &ReferenceData{
		Employees:       employees,
		EmpMap:          empMap,
		TagMap:          tagMap,
		JobMap:          jobMap,
		JobCCMap:        jobCCMap,
		EmpWorkHours:    empWHMap,
		RegionWorkHours: regionWHMap,
		TimeTypeMap:     ttMap,
	}, nil
}

func fetchRecords(db *gorm.DB, dateStr string, opts PrepareOptions, employees []models.Employee) ([]model.SupervisorRecord, []*model.ClockinRecord, error) {
	fmt.Println("Fetching records...")
	var supervisorRecords []model.SupervisorRecord
	supQuery := db.Where("date = ?", dateStr)
	if len(opts.Supervisors) > 0 {
		supQuery = supQuery.Where("supervisor_id IN ?", opts.Supervisors)
	}
	if len(opts.Employees) > 0 {
		supQuery = supQuery.Where("employee_id IN ?", opts.Employees)
	}

	if err := supQuery.Find(&supervisorRecords).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch supervisor records: %w", err)
	}

	var clockInRecords []*model.ClockinRecord
	clkQuery := db.Where("date = ?", dateStr)
	if len(opts.Employees) > 0 || len(opts.Supervisors) > 0 {
		var validTags []string
		for _, e := range employees {
			match := true
			if len(opts.Employees) > 0 {
				found := false
				for _, eid := range opts.Employees {
					if e.EmployeeID == eid {
						found = true
						break
					}
				}
				match = match && found
			}
			if len(opts.Supervisors) > 0 {
				found := false
				for _, sid := range opts.Supervisors {
					if e.ReportsToID == sid {
						found = true
						break
					}
				}
				match = match && found
			}

			if match && e.IdentificationTag != "" {
				validTags = append(validTags, e.IdentificationTag)
			}
		}

		if len(validTags) > 0 {
			clkQuery = clkQuery.Where("tag IN ?", validTags)
		} else {
			clkQuery = clkQuery.Where("1 = 0")
		}
	}

	if err := clkQuery.Find(&clockInRecords).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch clockin records: %w", err)
	}

	return supervisorRecords, clockInRecords, nil
}

func processClockInRecords(date time.Time, clockInRecords []*model.ClockinRecord, refData *ReferenceData, timesheetMap map[int32]model.OktediTimesheet) ([]string, []string) {
	var processedIDs []string
	var errorIDs []string

	groups := GroupRecords(clockInRecords)
	for _, g := range groups {
		groupIDs := make([]string, len(g.Records))
		for i, r := range g.Records {
			groupIDs[i] = r.ID
		}

		emp, ok := refData.TagMap[g.Tag]
		if !ok {
			fmt.Printf("Warning: No employee found for tag %s\n", g.Tag)
			errorIDs = append(errorIDs, groupIDs...)
			continue
		}

		startStr := g.GetClockIn()
		endStr := g.GetClockOut()

		if startStr == "" || endStr == "" {
			fmt.Printf("Warning: Incomplete clockin pairs for %s\n", g.Tag)
			errorIDs = append(errorIDs, groupIDs...)
			continue
		}

		startTime, err1 := utils.ParseISOTime(startStr)
		endTime, err2 := utils.ParseISOTime(endStr)

		startTime = utils.AdjustUtcToBrisbaneHours(startTime)
		endTime = utils.AdjustUtcToBrisbaneHours(endTime)

		if err1 != nil || err2 != nil {
			fmt.Printf("Warning: Failed to parse time for %s: %v, %v\n", g.Tag, err1, err2)
			errorIDs = append(errorIDs, groupIDs...)
			continue
		}

		ts := model.OktediTimesheet{
			EmployeeID:   emp.EmployeeID,
			Date:         date,
			Hours:        endTime.Sub(*startTime).Hours(),
			StartTime:    *startTime,
			FinishTime:   *endTime,
			ReviewStatus: "",
			Approved:     false,
			Break:        GetBreakMinutes(date, emp, refData.EmpWorkHours, refData.RegionWorkHours),
		}

		if emp.JobID != 0 {
			ts.ProjectID = utils.Ptr(emp.JobID)
		}
		if emp.CostCentreID != 0 {
			ts.CostCentreID = utils.Ptr(emp.CostCentreID)
		}

		timesheetMap[emp.EmployeeID] = ts
		processedIDs = append(processedIDs, groupIDs...)
	}

	return processedIDs, errorIDs
}

func applySupervisorRecords(date time.Time, supervisorRecords []model.SupervisorRecord, timesheetMap map[int32]model.OktediTimesheet, refData *ReferenceData) {
	// Sort by ID ascending so that later records overwrite earlier ones
	sort.Slice(supervisorRecords, func(i, j int) bool {
		return supervisorRecords[i].ID < supervisorRecords[j].ID
	})

	for _, rec := range supervisorRecords {
		empID := int32(rec.EmployeeId)
		ts, exists := timesheetMap[empID]
		if !exists {
			ts = model.OktediTimesheet{
				EmployeeID:   empID,
				Date:         date,
				ReviewStatus: "",
				Approved:     false,
				Break:        GetBreakMinutes(date, refData.EmpMap[empID], refData.EmpWorkHours, refData.RegionWorkHours),
			}
		}

		if rec.Clockin != nil && rec.Clockout != nil {
			duration := rec.Clockout.Sub(*rec.Clockin)
			ts.Hours = duration.Hours()
			ts.StartTime = *rec.Clockin
			ts.FinishTime = *rec.Clockout
		} else if !exists {
			// If creating new timesheet from supervisor record, use defined hours if available
			if def, found := GetDefinedWorkHours(date, refData.EmpMap[empID], refData.EmpWorkHours, refData.RegionWorkHours); found {
				// Use helper from timesheet_rules.go to parse
				// Since parseTimeOnDate is not exported, I'll implement a local helper or use AdjustTimesheetHours with zero actuals?
				// Actually, I'll just use a local parse helper for now to avoid side effects in timesheet_rules.go
				if start, err := ParseTimeOnDate(date, def.Start); err == nil {
					ts.StartTime = start
				}
				if finish, err := ParseTimeOnDate(date, def.Finish); err == nil {
					ts.FinishTime = finish
					if ts.FinishTime.Before(ts.StartTime) {
						ts.FinishTime = ts.FinishTime.Add(24 * time.Hour)
					}
				}
				if !ts.StartTime.IsZero() && !ts.FinishTime.IsZero() {
					ts.Hours = ts.FinishTime.Sub(ts.StartTime).Hours()
				}
			}
		}

		if rec.Project != "" {
			if job, ok := refData.JobMap[rec.Project]; ok {
				ts.ProjectID = utils.Ptr(job.JobID)
			}
		} else if !exists {
			if e, ok := refData.EmpMap[empID]; ok && e.JobID != 0 {
				ts.ProjectID = utils.Ptr(e.JobID)
			}
		}

		if rec.Wbs != "" {
			if ts.ProjectID != nil {
				if jobCCs, ok := refData.JobCCMap[*ts.ProjectID]; ok {
					if cc, ok := jobCCs[rec.Wbs]; ok {
						ts.CostCentreID = utils.Ptr(cc.CostCentreID)
					}
				}
			}
		} else if !exists {
			if e, ok := refData.EmpMap[empID]; ok && e.CostCentreID != 0 {
				ts.CostCentreID = utils.Ptr(e.CostCentreID)
			}
		}

		timesheetMap[empID] = ts
	}
}

func persistTimesheets(db *gorm.DB, dateStr string, timesheetMap map[int32]model.OktediTimesheet, summary *PrepareSummary) error {
	fmt.Printf("Saving %d timesheets to DB...\n", len(timesheetMap))
	if len(timesheetMap) == 0 {
		return nil
	}

	var empIDs []int32
	for id := range timesheetMap {
		empIDs = append(empIDs, id)
	}

	var existingTimesheets []model.OktediTimesheet
	if err := db.Where("date = ? AND employee_id IN ?", dateStr, empIDs).Find(&existingTimesheets).Error; err != nil {
		return fmt.Errorf("failed to fetch existing timesheets: %w", err)
	}

	existingMap := make(map[int32]model.OktediTimesheet)
	for _, et := range existingTimesheets {
		existingMap[et.EmployeeID] = et
	}

	var timesheets []model.OktediTimesheet
	for _, ts := range timesheetMap {
		if existing, exists := existingMap[ts.EmployeeID]; exists {
			// Never overwrite a row that's already approved (manual or a prior
			// auto-approve) — preserve the approval and any edits.
			if existing.Approved {
				if summary != nil {
					summary.KeptApproved++
				}
				continue
			}
			ts.ID = existing.ID
			ts.TimesheetID = existing.TimesheetID
			ts.ProjectID = existing.ProjectID
			ts.CostCentreID = existing.CostCentreID
			if shouldPreserveAbsent(existing) {
				if summary != nil {
					summary.KeptAbsent++
				}
				continue
			}
			if summary != nil {
				summary.Recomputed++
			}
		} else if summary != nil {
			summary.New++
		}
		// Save everything else, including rows just auto-approved this run.
		timesheets = append(timesheets, ts)
	}

	if len(timesheets) == 0 {
		return nil
	}

	if err := db.Save(&timesheets).Error; err != nil {
		return fmt.Errorf("failed to save timesheets: %w", err)
	}

	return nil
}

func updateProcessStatuses(db *gorm.DB, processedIDs, skippedIDs, errorIDs []string) {
	fmt.Printf("Updating statuses: Processed=%d, Skipped=%d, Error=%d\n", len(processedIDs), len(skippedIDs), len(errorIDs))

	if len(processedIDs) > 0 {
		db.Model(&model.ClockinRecord{}).Where("id IN ?", processedIDs).Update("process_status", "processed")
	}
	if len(skippedIDs) > 0 {
		db.Model(&model.ClockinRecord{}).Where("id IN ?", skippedIDs).Update("process_status", "skipped")
	}
	if len(errorIDs) > 0 {
		db.Model(&model.ClockinRecord{}).Where("id IN ?", errorIDs).Update("process_status", "error")
	}
}

func removeSeconds(timesheetMap map[int32]model.OktediTimesheet) {
	for empID, ts := range timesheetMap {
		ts.StartTime = ts.StartTime.Truncate(time.Minute)
		ts.FinishTime = ts.FinishTime.Truncate(time.Minute)

		// Recalculate hours
		duration := ts.FinishTime.Sub(ts.StartTime)
		ts.Hours = math.Max(0, duration.Hours())
		timesheetMap[empID] = ts
	}
}

func applySnappingRules(timesheetMap map[int32]model.OktediTimesheet, refData *ReferenceData) {
	for empID, ts := range timesheetMap {
		emp, ok := refData.EmpMap[empID]
		if !ok {
			continue
		}

		// Apply snapping rules (15m early / 10m late etc)
		adjusted, err := AdjustTimesheetHours(ts.StartTime, ts.FinishTime, emp, refData.EmpWorkHours, refData.RegionWorkHours)
		if err != nil {
			fmt.Printf("Warning: Failed to adjust times for employee %d: %v\n", empID, err)
		} else {
			ts.StartTime = adjusted.StartTime
			ts.FinishTime = adjusted.FinishTime
		}

		// Recalculate hours after snapping
		duration := ts.FinishTime.Sub(ts.StartTime)
		ts.Hours = math.Max(0, duration.Hours())
		timesheetMap[empID] = ts
	}
}

func applyBreaks(timesheetMap map[int32]model.OktediTimesheet) {
	for empID, ts := range timesheetMap {
		if ts.Break != nil && *ts.Break > 0 {
			breakHours := float64(*ts.Break) / 60.0
			// Deduct break only if total hours greater than break time
			if ts.Hours > breakHours {
				ts.Hours -= breakHours
				timesheetMap[empID] = ts
			}
		}
	}
}

func updateReviewStatus(date time.Time, timesheetMap map[int32]model.OktediTimesheet, refData *ReferenceData) {
	for empID, ts := range timesheetMap {
		emp, ok := refData.EmpMap[empID]
		if !ok {
			continue
		}

		var timeType *models.PayrollTimeType
		if emp.RosterPayrollTimeTypeID != 0 {
			if tt, ok := refData.TimeTypeMap[emp.RosterPayrollTimeTypeID]; ok {
				timeType = &tt
			}
		}

		// Missing-roster guard — highest priority. A roster employee with an
		// invalid setup can't be trusted by IsRosteredOn (it fails open), so flag it.
		if isRoster, valid, reason := ValidateRoster(emp, timeType); isRoster && !valid {
			fmt.Printf("Warning: employee %d roster misconfigured (%s) — marking missing-roster\n", empID, reason)
			ts.ReviewStatus = "missing-roster"
			timesheetMap[empID] = ts
			continue
		}

		// Not-rostered guard
		if !IsRosteredOn(emp, timeType, date) {
			ts.ReviewStatus = "not-rostered"
			timesheetMap[empID] = ts
			continue
		}

		// Absent rows — leave their status as-is (Hours=0 is expected)
		if ts.ReviewStatus == "absent" {
			continue
		}

		// Layer 1: normal review status.
		UpdateSingleReviewStatus(&ts, emp, refData.EmpWorkHours, refData.RegionWorkHours)

		// Auto-approve when the adjusted span matches the rostered span
		// (Rostered == Adjusted). UpdateSingleReviewStatus leaves an empty
		// ReviewStatus precisely in that matched case.
		if ts.ReviewStatus == "" {
			ts.Approved = true
		}

		timesheetMap[empID] = ts
	}
}

func UpdateSingleReviewStatus(
	ts *model.OktediTimesheet,
	emp models.Employee,
	empWorkHours map[int32]map[int32]models.EmployeeWorkHour,
	regionWorkHours map[int32]map[int32]models.RegionWorkHour,
) {
	// If no project assigned, mark as required
	if ts.ProjectID == nil {
		ts.ReviewStatus = "required"
		return
	}

	if emp.JobID != 0 && *ts.ProjectID != emp.JobID {
		ts.ReviewStatus = "required"
		return
	}

	if emp.CostCentreID != 0 && (ts.CostCentreID == nil || *ts.CostCentreID != emp.CostCentreID) {
		ts.ReviewStatus = "required"
		return
	}

	def, found := GetDefinedWorkHours(ts.StartTime, emp, empWorkHours, regionWorkHours)
	if !found {
		ts.ReviewStatus = "required"
		return
	}

	dateBase := time.Date(ts.StartTime.Year(), ts.StartTime.Month(), ts.StartTime.Day(), 0, 0, 0, 0, ts.StartTime.Location())
	defStart, err1 := ParseTimeOnDate(dateBase, def.Start)
	defFinish, err2 := ParseTimeOnDate(dateBase, def.Finish)

	if err1 != nil || err2 != nil {
		ts.ReviewStatus = "required"
		return
	}

	if defFinish.Before(defStart) {
		defFinish = defFinish.Add(24 * time.Hour)
	}

	expectedHours := defFinish.Sub(defStart).Hours()
	if expectedHours < 0 {
		expectedHours = 0
	}

	actualTotal := ts.Hours
	if ts.Break != nil && *ts.Break > 0 {
		actualTotal += float64(*ts.Break) / 60.0
	}

	// Use a small epsilon for float comparison to avoid precision issues
	if math.Abs(actualTotal-expectedHours) > 0.001 {
		ts.ReviewStatus = "required"
	} else {
		ts.ReviewStatus = ""
	}
}

// shouldPreserveAbsent returns true if an existing absent-tagged timesheet
// should NOT be overwritten by re-prepare (i.e. a supervisor has already edited it).
func shouldPreserveAbsent(existing model.OktediTimesheet) bool {
	if existing.ReviewStatus != "absent" {
		return false
	}
	// Default absent state: Hours=0 and not approved — safe to overwrite
	if existing.Hours == 0 && !existing.Approved {
		return false
	}
	return true // supervisor added hours or approved the row
}

func matchesFilter(emp models.Employee, opts PrepareOptions) bool {
	if len(opts.Employees) > 0 {
		found := false
		for _, eid := range opts.Employees {
			if emp.EmployeeID == eid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(opts.Supervisors) > 0 {
		found := false
		for _, sid := range opts.Supervisors {
			if emp.ReportsToID == sid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func injectAbsentRows(date time.Time, employees []models.Employee, opts PrepareOptions, timesheetMap map[int32]model.OktediTimesheet, refData *ReferenceData) {
	endDateThreshold := date.AddDate(0, 0, -7)
	for _, emp := range employees {
		if !matchesFilter(emp, opts) {
			continue
		}
		if !emp.EndDate.IsZero() && emp.EndDate.Before(endDateThreshold) {
			continue
		}
		if _, exists := timesheetMap[emp.EmployeeID]; exists {
			continue
		}
		var timeType *models.PayrollTimeType
		if emp.RosterPayrollTimeTypeID != 0 {
			if tt, ok := refData.TimeTypeMap[emp.RosterPayrollTimeTypeID]; ok {
				timeType = &tt
			}
		}

		isRoster, valid, reason := ValidateRoster(emp, timeType)
		// Absent rows are only for roster employees — skip non-roster staff.
		if !isRoster {
			continue
		}

		reviewStatus := "absent"
		if !valid {
			// Invalid roster setup — we can't trust IsRosteredOn (it fails open),
			// so inject a flagged row instead of a spurious "absent" one.
			fmt.Printf("Warning: employee %d roster misconfigured (%s) — injecting missing-roster row\n", emp.EmployeeID, reason)
			reviewStatus = "missing-roster"
		} else if !IsRosteredOn(emp, timeType, date) {
			// Roster valid but not rostered on this date — skip, don't inject.
			continue
		}

		ts := model.OktediTimesheet{
			EmployeeID:   emp.EmployeeID,
			Date:         date,
			Hours:        0,
			ReviewStatus: reviewStatus,
			Approved:     false,
			Break:        GetBreakMinutes(date, emp, refData.EmpWorkHours, refData.RegionWorkHours),
		}
		if emp.JobID != 0 {
			ts.ProjectID = utils.Ptr(emp.JobID)
		}
		if emp.CostCentreID != 0 {
			ts.CostCentreID = utils.Ptr(emp.CostCentreID)
		}
		timesheetMap[emp.EmployeeID] = ts
	}
}

func RefreshReviewStatus(db *gorm.DB, ts *model.OktediTimesheet) error {
	var emp models.Employee
	if err := db.First(&emp, ts.EmployeeID).Error; err != nil {
		return err
	}

	empWHMap := make(map[int32]map[int32]models.EmployeeWorkHour)
	var empWorkHours []models.EmployeeWorkHour
	if err := db.Where("EmployeeId = ?", emp.EmployeeID).Find(&empWorkHours).Error; err == nil {
		for _, wh := range empWorkHours {
			if _, ok := empWHMap[wh.EmployeeID]; !ok {
				empWHMap[wh.EmployeeID] = make(map[int32]models.EmployeeWorkHour)
			}
			empWHMap[wh.EmployeeID][wh.DayOfWeek] = wh
		}
	}

	regionWHMap := make(map[int32]map[int32]models.RegionWorkHour)
	var regionWorkHours []models.RegionWorkHour
	if err := db.Where("CalendarRegionId = ?", emp.CalendarRegionID).Find(&regionWorkHours).Error; err == nil {
		for _, wh := range regionWorkHours {
			if _, ok := regionWHMap[wh.CalendarRegionID]; !ok {
				regionWHMap[wh.CalendarRegionID] = make(map[int32]models.RegionWorkHour)
			}
			regionWHMap[wh.CalendarRegionID][wh.DayOfWeek] = wh
		}
	}

	UpdateSingleReviewStatus(ts, emp, empWHMap, regionWHMap)
	return nil
}
