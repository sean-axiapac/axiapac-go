package main

import (
	"fmt"
	"log"
	"time"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := "root:development@tcp(localhost:3306)/oktedi?parseTime=true"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	startDate := time.Date(2025, 12, 22, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 12, 28, 0, 0, 0, 0, time.UTC)

	// mockOktediTimesheets(db, startDate, endDate)
	mockClockinRecords(db, startDate, endDate)
}

func mockOktediTimesheets(db *gorm.DB, startDate, endDate time.Time) {

	// Fetch employees in the specified range
	var employees []models.Employee
	if err := db.Where("EmployeeId BETWEEN ? AND ?", 101, 300).Find(&employees).Error; err != nil {
		log.Fatalf("failed to fetch employees: %v", err)
	}

	var timesheets []model.OktediTimesheet

	for _, emp := range employees {
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			ts := model.OktediTimesheet{
				EmployeeID:   emp.EmployeeID,
				Date:         d,
				Hours:        8.0,
				ReviewStatus: "",
				Approved:     false,
			}

			if emp.JobID != 0 {
				ts.ProjectID = utils.Ptr(emp.JobID)
			}
			if emp.CostCentreID != 0 {
				ts.CostCentreID = utils.Ptr(emp.CostCentreID)
			}

			timesheets = append(timesheets, ts)
		}
	}

	if len(timesheets) == 0 {
		fmt.Println("No employees found in range 101-300. No timesheets to insert.")
		return
	}

	fmt.Printf("Inserting %d mock timesheets for %d employees...\n", len(timesheets), len(employees))

	// Batch insert (chunk size 100 to be safe)
	if err := db.CreateInBatches(timesheets, 100).Error; err != nil {
		log.Fatalf("failed to insert mock timesheets: %v", err)
	}

	fmt.Println("Successfully inserted mock timesheets.")
}

func mockClockinRecords(db *gorm.DB, startDate, endDate time.Time) {
	// Fetch employees in the specified range
	var employees []models.Employee
	if err := db.Where("EmployeeId BETWEEN ? AND ?", 101, 300).Find(&employees).Error; err != nil {
		log.Fatalf("failed to fetch employees for clockin: %v", err)
	}

	var records []model.ClockinRecord

	for _, emp := range employees {
		if emp.IdentificationTag == "" {
			continue // skip employees without tag
		}

		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			dateStr := d.Format("2006-01-02")

			// Clock In
			records = append(records, model.ClockinRecord{
				ID:            uuid.New().String(),
				Tag:           emp.IdentificationTag,
				Date:          dateStr,
				Kind:          "in",
				Timestamp:     d.Add(8 * time.Hour).Format(time.RFC3339),
				ProcessStatus: "pending",
			})

			// Clock Out
			records = append(records, model.ClockinRecord{
				ID:            uuid.New().String(),
				Tag:           emp.IdentificationTag,
				Date:          dateStr,
				Kind:          "out",
				Timestamp:     d.Add(16 * time.Hour).Format(time.RFC3339),
				ProcessStatus: "pending",
			})
		}
	}

	if len(records) == 0 {
		fmt.Println("No records to insert for clockin.")
		return
	}

	fmt.Printf("Inserting %d mock clockin records for %d employees...\n", len(records), len(employees))

	// Batch insert
	if err := db.CreateInBatches(records, 100).Error; err != nil {
		log.Fatalf("failed to insert mock clockin records: %v", err)
	}

	fmt.Println("Successfully inserted mock clockin records.")
}
