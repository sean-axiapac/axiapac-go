package handlers

import (
	"net/http"
	"time"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func WatermelonPushHandler(dm *core.DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var push WatermelonPush

		// Parse JSON body
		if err := c.ShouldBindJSON(&push); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := dm.Exec(c.Request.Context(), "oktedi", func(db *gorm.DB) error {
			if err := BulkUpsertEmployees(db, push.Changes.Records.Created); err != nil {
				return err
			}
			if err := BulkUpsertEmployees(db, push.Changes.Records.Updated); err != nil {
				return err
			}
			// if err := BulkUpsertEmployees(db, push.Changes.Records.Deleted); err != nil {
			// 	return err
			// }
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
			"lastPushedAt": time.Now().Unix(),
		}))
	}
}

func BulkUpsertEmployees(db *gorm.DB, employees []EmployeeClockInRecord) error {

	// Bulk upsert
	if len(employees) > 0 {
		records := utils.Map(employees, func(e EmployeeClockInRecord) model.ClockinRecord {
			date := e.Timestamp.In(utils.BrisbaneTZ)
			return model.ClockinRecord{
				ID:        e.ID,
				Tag:       e.Tag,
				Kind:      e.Kind,
				Timestamp: e.Timestamp.Format(time.RFC3339),
				Date:      date.Format("2006-01-02"),
				CardID:    e.CardID,
				DeviceID:  e.DeviceID,

				Status:  e.Status,
				Changed: e.Changed,

				ProcessStatus: "pending",
			}
		})
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}}, // conflict key
			UpdateAll: true,                          // update all fields on conflict
		}).Create(&records).Error; err != nil {
			return err
		}
	}

	return nil
}
