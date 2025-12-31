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

func ProcessClockInRecords(db *gorm.DB, date time.Time) error {
	return ProcessClockInRecordsWithFilters(db, date, PrepareOptions{
		StartDate: date,
		EndDate:   date,
	})
}

func ProcessClockInRecordsWithFilters(db *gorm.DB, date time.Time, opts PrepareOptions) error {
	dateStr := date.Format("2006-01-02")

	// 1. Fetch Reference Data
	fmt.Println("Fetching reference data...")
	var employees []models.Employee
	if err := db.Find(&employees).Error; err != nil {
		return fmt.Errorf("failed to fetch employees: %w", err)
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
		return fmt.Errorf("failed to fetch jobs: %w", err)
	}
	jobMap := make(map[string]models.Job)
	for _, j := range jobs {
		jobMap[j.JobNo] = j
	}

	var costCentres []models.CostCentre
	if err := db.Find(&costCentres).Error; err != nil {
		return fmt.Errorf("failed to fetch cost centres: %w", err)
	}
	ccMap := make(map[string]models.CostCentre)
	for _, cc := range costCentres {
		ccMap[cc.Code] = cc
	}

	// 2. Fetch Records
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
		return fmt.Errorf("failed to fetch supervisor records: %w", err)
	}

	var clockInRecords []*model.ClockinRecord
	clkQuery := db.Where("date = ?", dateStr)
	// For clockin records, we only have the tag. We need to filter by employees if specified.
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
		return fmt.Errorf("failed to fetch clockin records: %w", err)
	}

	// 3. Process Records
	// Map EmployeeID -> OktediTimesheet
	timesheetMap := make(map[int32]model.OktediTimesheet)

	// Track ClockIn IDs for status updates
	processedClockInIDs := make([]string, 0)
	skippedClockInIDs := make([]string, 0)
	errorClockInIDs := make([]string, 0)

	// Process Supervisor Records (Precedence: High)
	// Sort by ID ascending so that later records overwrite earlier ones (Latest wins)
	sort.Slice(supervisorRecords, func(i, j int) bool {
		return supervisorRecords[i].ID < supervisorRecords[j].ID
	})

	for _, rec := range supervisorRecords {
		empID := int32(rec.EmployeeId)

		var hours float64
		if rec.Clockin != nil && rec.Clockout != nil {
			duration := rec.Clockout.Sub(*rec.Clockin)
			hours = duration.Hours()
		}

		ts := model.OktediTimesheet{
			EmployeeID:   empID,
			Date:         date,
			Hours:        hours,
			ReviewStatus: "",
			Approved:     false, // Assuming supervisor records are approved
		}

		// Map Project to JobID
		if job, ok := jobMap[rec.Project]; ok {
			ts.ProjectID = utils.Ptr(job.JobID)
		} else if e, ok := empMap[empID]; ok && e.JobID != 0 {
			ts.ProjectID = utils.Ptr(e.JobID)
		}

		// Map Wbs to CostCentreID
		if cc, ok := ccMap[rec.Wbs]; ok {
			ts.CostCentreID = utils.Ptr(cc.CostCentreID)
		} else if e, ok := empMap[empID]; ok && e.CostCentreID != 0 {
			ts.CostCentreID = utils.Ptr(e.CostCentreID)
		}

		timesheetMap[empID] = ts
	}

	// Process ClockIn Records (Precedence: Low)
	// Group by Tag
	groups := GroupRecords(clockInRecords)

	for _, g := range groups {
		// Collect all IDs in this group
		groupIDs := make([]string, len(g.Records))
		for i, r := range g.Records {
			groupIDs[i] = r.ID
		}

		emp, ok := tagMap[g.Tag]
		if !ok {
			fmt.Printf("Warning: No employee found for tag %s\n", g.Tag)
			errorClockInIDs = append(errorClockInIDs, groupIDs...)
			continue
		}

		// If we already have a timesheet from Supervisor, SKIP
		if _, exists := timesheetMap[emp.EmployeeID]; exists {
			skippedClockInIDs = append(skippedClockInIDs, groupIDs...)
			continue
		}

		startStr := g.GetClockIn()
		endStr := g.GetClockOut()

		if startStr == "" || endStr == "" {
			fmt.Printf("Warning: Incomplete clockin pairs for %s\n", g.Tag)
			errorClockInIDs = append(errorClockInIDs, groupIDs...)
			continue
		}

		startTime, err1 := utils.ParseISOTime(startStr)
		endTime, err2 := utils.ParseISOTime(endStr)

		if err1 != nil || err2 != nil {
			fmt.Printf("Warning: Failed to parse time for %s: %v, %v\n", g.Tag, err1, err2)
			errorClockInIDs = append(errorClockInIDs, groupIDs...)
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
		processedClockInIDs = append(processedClockInIDs, groupIDs...)
	}

	// 4. Persist to DB using Upsert
	fmt.Printf("Saving %d timesheets to DB...\n", len(timesheetMap))
	if len(timesheetMap) > 0 {
		var existingTimesheets []model.OktediTimesheet
		var empIDs []int32
		for id := range timesheetMap {
			empIDs = append(empIDs, int32(id))
		}
		if err := db.Where("date = ? AND employee_id IN ?", dateStr, empIDs).Find(&existingTimesheets).Error; err != nil {
			return fmt.Errorf("failed to fetch existing timesheets: %w", err)
		}

		existingMap := make(map[int32]int32) // EmployeeID -> ID
		for _, et := range existingTimesheets {
			existingMap[et.EmployeeID] = et.ID
		}

		var timesheets []model.OktediTimesheet
		for _, ts := range timesheetMap {
			if id, exists := existingMap[ts.EmployeeID]; exists {
				ts.ID = id // Set ID to trigger Update
			}
			timesheets = append(timesheets, ts)
		}

		if err := db.Save(&timesheets).Error; err != nil {
			return fmt.Errorf("failed to save timesheets: %w", err)
		}
	}

	// 5. Update Status of ClockIn Records
	fmt.Printf("Updating statuses: Processed=%d, Skipped=%d, Error=%d\n", len(processedClockInIDs), len(skippedClockInIDs), len(errorClockInIDs))

	if len(processedClockInIDs) > 0 {
		if err := db.Model(&model.ClockinRecord{}).Where("id IN ?", processedClockInIDs).Update("process_status", "processed").Error; err != nil {
			fmt.Printf("Error updating processed status: %v\n", err)
		}
	}
	if len(skippedClockInIDs) > 0 {
		if err := db.Model(&model.ClockinRecord{}).Where("id IN ?", skippedClockInIDs).Update("process_status", "skipped").Error; err != nil {
			fmt.Printf("Error updating skipped status: %v\n", err)
		}
	}
	if len(errorClockInIDs) > 0 {
		if err := db.Model(&model.ClockinRecord{}).Where("id IN ?", errorClockInIDs).Update("process_status", "error").Error; err != nil {
			fmt.Printf("Error updating error status: %v\n", err)
		}
	}

	fmt.Println("Done.")
	return nil
}
