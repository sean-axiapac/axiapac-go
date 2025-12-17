package main

import (
	"fmt"
	"os"

	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {

	dsn := os.Getenv("DSN") //"root:development@tcp(localhost:3306)/oktedi?parseTime=true"
	// dsn := "axiapac:Tingalpa2019@tcp(experimental.cixs43nsk6u5.ap-southeast-2.rds.amazonaws.com:3306)/oktedi?parseTime=true"
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	// get employees with id from 100 to 300
	var employees []models.Employee
	if err := db.Where("employeeid BETWEEN ? AND ?", 100, 300).Find(&employees).Error; err != nil {
		panic(err)
	}

	// create a slice of model.OktediTimesheet
	var timesheets []model.OktediTimesheet

	// loop from 100 to 300
	for _, emp := range employees {
		// loop from 6 Oct 2025 to 19 Oct 2025
		for j := 6; j <= 19; j++ {
			timesheets = append(timesheets, model.OktediTimesheet{
				EmployeeID:   emp.EmployeeID,
				Date:         utils.MustParseDate(fmt.Sprintf("2025-10-%02d", j)),
				Hours:        8.0,
				ReviewStatus: "",
				Approved:     false,
				ProjectID:    utils.Ptr(int32(1962)),
				CostCentreId: utils.Ptr(int32(99351)),
			})
		}
	}

	// bulk insert timesheets
	if err := db.CreateInBatches(timesheets, 100).Error; err != nil {
		panic(err)
	}

}
