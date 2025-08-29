package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/lambdas/clockin/helper"
)

func getFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	// caller is responsible for closing if needed
	return file, nil
}

var DSN = "root:development@tcp(localhost:3306)/development?parseTime=true"

func main() {
	// Create a buffer to hold the S3 file data
	// var stream bytes.Buffer
	// err := filesystem.ReadFile("my-bucket", "path/to/my/file.csv", context.Background(), &stream)
	fmt.Printf("Fetching file\n")
	stream, err := getFile("lambdas/clockin/test.csv")
	if err != nil {
		log.Fatalf("failed to read file from S3: %v", err)
	}
	defer stream.Close()

	fmt.Printf("Parsing CSV\n")
	records, err := helper.ParseClockInCSV(stream, 10*3600) // UTC+10
	if err != nil {
		log.Fatalf("failed to parse CSV: %v", err)
	}
	fmt.Printf("Parsed %d records\n", len(records))
	grouped := helper.GroupRecords(records)
	fmt.Printf("Grouped %d records\n", len(grouped))

	db := core.ConnectDB(DSN)

	for _, group := range grouped {
		fmt.Printf("  ID: %s, Date: %s, From: %s, To: %s\n",
			group.UserID, group.Date, group.From, group.To)

		// find employee
		employeeId, err := strconv.Atoi(group.UserID)
		if err != nil {
			fmt.Printf("Error converting UserID to int: %v\n", err)
			continue
		}
		employee, err := core.FindEmployeeByID(db, employeeId)
		if err != nil {
			fmt.Printf("Error finding employee by ID: %v\n", err)
			continue
		}
		fmt.Printf("Found employee: %s\n", employee.Code)

		// TODO: create timesheet

		// TODO: save/update timesheet
	}
	fmt.Printf("Completed\n")
}
