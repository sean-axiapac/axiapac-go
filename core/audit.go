package core

import (
	"fmt"
	"time"

	"axiapac.com/axiapac/core/models"
	"gorm.io/gorm"
)

// AuditChange adds a history record and an audit record for a change.
// action: 1 = Insert, 2 = Update, 3 = Delete
func AuditChange(db *gorm.DB, userID int32, tableName string, tableKey int32, action int32, ipAddress string, title string, description string, auditChangeMessage string) error {
	history := models.History{
		UserID:      userID,
		CreatedAt:   time.Now(),
		TableName_:  tableName,
		TableKey:    tableKey,
		Action:      action,
		IPAddress:   ipAddress,
		GeoCode:     "",
		Title:       title,
		Description: description,
	}

	if err := db.Create(&history).Error; err != nil {
		return err
	}

	audit := models.Audit{
		AuditID: history.HistoryID,
		Changes: fmt.Sprintf(`%s`, auditChangeMessage),
	}

	return db.Create(&audit).Error
}
