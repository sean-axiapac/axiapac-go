package console

import (
	"context"
	"fmt"
	"time"

	"axiapac.com/axiapac/infrastructure/devops"
	"axiapac.com/axiapac/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(ctx context.Context) (*gorm.DB, error) {
	databases, err := devops.LoadDBConfig(ctx)
	if err != nil {
		return nil, err

	}

	dbconfig := utils.Find(databases, func(db devops.DBEntry) bool {
		return db.Name == "console"
	})
	if dbconfig == nil {
		return nil, fmt.Errorf("console database parameter not found")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbconfig.Username,
		dbconfig.Password,
		dbconfig.Host,
		dbconfig.Name,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Info),
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Optional: configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
