package main

import (
	"log"
	"os"

	"axiapac.com/axiapac/oktedi/model"
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

	models := []interface{}{
		&model.ClockinRecord{},
		&model.SupervisorRecord{},
		&model.OktediTimesheet{},
	}

	for _, m := range models {
		if !db.Migrator().HasTable(m) {
			err := db.Migrator().CreateTable(m)
			if err != nil {
				log.Fatalf("failed to create table for %T: %v", m, err)
			}
		}
	}

}
