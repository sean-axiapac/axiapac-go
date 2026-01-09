package core

import (
	"fmt"
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

func Prepare(db *gorm.DB, opts PrepareOptions) error {
	// iterate through each day in the range
	for d := opts.StartDate; !d.After(opts.EndDate); d = d.AddDate(0, 0, 1) {
		if err := ProcessClockInRecordsWithFilters(db, d, opts); err != nil {
			return err
		}
	}
	return nil
}

type ReferenceData struct {
	Employees []models.Employee
	EmpMap    map[int32]models.Employee
	TagMap    map[string]models.Employee
	JobMap    map[string]models.Job
	JobCCMap  map[int32]map[string]models.CostCentre
}

func ProcessClockInRecordsWithFilters(db *gorm.DB, date time.Time, opts PrepareOptions) error {
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

	// Track ClockIn IDs for status updates
	processedClockInIDs, errorClockInIDs := processClockInRecords(date, clockInRecords, refData.TagMap, timesheetMap)

	// Apply Supervisor Records (Overwrites)
	applySupervisorRecords(date, supervisorRecords, timesheetMap, refData)

	// 4. Persist to DB
	if err := persistTimesheets(db, dateStr, timesheetMap); err != nil {
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

	return &ReferenceData{
		Employees: employees,
		EmpMap:    empMap,
		TagMap:    tagMap,
		JobMap:    jobMap,
		JobCCMap:  jobCCMap,
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

func processClockInRecords(date time.Time, clockInRecords []*model.ClockinRecord, tagMap map[string]models.Employee, timesheetMap map[int32]model.OktediTimesheet) ([]string, []string) {
	var processedIDs []string
	var errorIDs []string

	groups := GroupRecords(clockInRecords)
	for _, g := range groups {
		groupIDs := make([]string, len(g.Records))
		for i, r := range g.Records {
			groupIDs[i] = r.ID
		}

		emp, ok := tagMap[g.Tag]
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

		if err1 != nil || err2 != nil {
			fmt.Printf("Warning: Failed to parse time for %s: %v, %v\n", g.Tag, err1, err2)
			errorIDs = append(errorIDs, groupIDs...)
			continue
		}

		hours := endTime.Sub(*startTime).Hours()

		ts := model.OktediTimesheet{
			EmployeeID:   emp.EmployeeID,
			Date:         date,
			Hours:        hours,
			ReviewStatus: "",
			Approved:     false,
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
			}
		}

		if rec.Clockin != nil && rec.Clockout != nil {
			duration := rec.Clockout.Sub(*rec.Clockin)
			ts.Hours = duration.Hours()
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

func persistTimesheets(db *gorm.DB, dateStr string, timesheetMap map[int32]model.OktediTimesheet) error {
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
			ts.ID = existing.ID
			ts.TimesheetID = existing.TimesheetID
			ts.Approved = existing.Approved // Preserve Approved status
		}
		timesheets = append(timesheets, ts)
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
