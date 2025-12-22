package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Parse CLI flags
	// dateStr := flag.String("date", "", "Date to process (YYYY-MM-DD). Defaults to yesterday.")
	// flag.Parse()
	dateStr := utils.Ptr("2025-12-17")

	// Calculate target date
	var targetDate time.Time
	if *dateStr != "" {
		var err error
		targetDate, err = time.Parse("2006-01-02", *dateStr)
		if err != nil {
			panic(fmt.Sprintf("Invalid date format: %v", err))
		}
	} else {
		// Default to yesterday
		targetDate = time.Now().AddDate(0, 0, -1)
	}

	fmt.Printf("Processing records for date: %s\n", targetDate.Format("2006-01-02"))

	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = "root:development@tcp(localhost:3306)/oktedi?parseTime=true"
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(err)
	}

	if err := Run(db, targetDate); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func Run(db *gorm.DB, date time.Time) error {
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
	if err := db.Where("date = ?", dateStr).Find(&supervisorRecords).Error; err != nil {
		return fmt.Errorf("failed to fetch supervisor records: %w", err)
	}

	var clockInRecords []*model.ClockinRecord
	if err := db.Where("date = ? AND process_status = ?", dateStr, "pending").Find(&clockInRecords).Error; err != nil {
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
			Approved:     true, // Assuming supervisor records are approved
		}

		// Map Project to JobID
		if job, ok := jobMap[rec.Project]; ok {
			ts.ProjectID = utils.Ptr(job.JobID)
		}

		// Map Wbs to CostCentreID
		if cc, ok := ccMap[rec.Wbs]; ok {
			ts.CostCentreId = utils.Ptr(cc.CostCentreID)
		}

		timesheetMap[empID] = ts
	}

	// Process ClockIn Records (Precedence: Low)
	// Group by Tag
	groups := PrepareRecords(clockInRecords)

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

// Helpers for ClockInRecords (Simulating createtimesheet logic)

type RecordGroup struct {
	Tag     string
	Date    string
	Records []*model.ClockinRecord
}

func (rg *RecordGroup) GetClockIn() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[0].Timestamp
}

func (rg *RecordGroup) GetClockOut() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[len(rg.Records)-1].Timestamp
}

func PrepareRecords(records []*model.ClockinRecord) []*RecordGroup {
	// group by date - although we are processing single date, the util is generic
	var groups []*RecordGroup
	dategroups := utils.GroupBy(records, func(r *model.ClockinRecord) string { return r.Date })

	for date, recs := range dategroups {
		// group by tag
		taggroups := utils.GroupBy(recs, func(r *model.ClockinRecord) string { return r.Tag })
		for tag, r2 := range taggroups {
			// Sort records by timestamp to ensure First and Last are correct
			sort.Slice(r2, func(i, j int) bool {
				return r2[i].Timestamp < r2[j].Timestamp
			})

			rg := &RecordGroup{
				Tag:     tag,
				Date:    date,
				Records: r2,
			}
			groups = append(groups, rg)
		}
	}
	return groups
}
