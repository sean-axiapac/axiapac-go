package handlers

import (
	"net/http"
	"strconv"
	"time"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/utils"
	"axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssignmentDTO struct {
	ID         int32      `json:"id"`
	EmployeeId int        `json:"employeeId"`
	Project    string     `json:"project"`
	Wbs        string     `json:"wbs"`
	Date       string     `json:"date"`
	Clockin    *time.Time `json:"clockin"`
	Clockout   *time.Time `json:"clockout"`
	DeviceID   string     `json:"deviceId"`
}

func SearchSupervisorRecordsHandler(dm *core.DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var records []model.SupervisorRecord
		// add supervisorid to route parameter
		supervisorId := c.Param("supervisorId")
		date := c.Query("date")
		if date != "" {
			if err := dm.Exec(c.Request.Context(), "oktedi", func(db *gorm.DB) error {
				return db.Where("date = ? AND supervisor_id = ?", date, supervisorId).Find(&records).Error
			}); err != nil {
				c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
				return
			}
			c.JSON(http.StatusOK, common.NewSuccessResponse(records))
			return
		}

		// if no date filter, return all records (limit to 1000)
		if err := dm.Exec(c.Request.Context(), "oktedi", func(db *gorm.DB) error {
			return db.Limit(1000).Find(&records).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
			return
		}

		c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
			"records": records,
		}))
	}
}

func SaveSupervisorRecordsHandler(dm *core.DatabaseManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data []AssignmentDTO
		idStr := c.Param("supervisorId")

		supervisorId, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
			return
		}

		// Parse JSON body
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.FormatBindingError(err)))
			return
		}

		if err := dm.Exec(c.Request.Context(), "oktedi", func(db *gorm.DB) error {
			if err := BulkUpsert(db, supervisorId, data); err != nil {
				return err
			}

			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{}))
	}
}

func BulkUpsert(db *gorm.DB, supervisorId int, records []AssignmentDTO) error {

	// Bulk upsert
	if len(records) > 0 {
		records := utils.Map(records, func(e AssignmentDTO) model.SupervisorRecord {
			return model.SupervisorRecord{
				ID:           e.ID,
				SupervisorId: supervisorId,
				EmployeeId:   e.EmployeeId,
				Project:      e.Project,
				Wbs:          e.Wbs,
				Date:         e.Date,
				Clockin:      e.Clockin,
				Clockout:     e.Clockout,
				DeviceID:     e.DeviceID,
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
