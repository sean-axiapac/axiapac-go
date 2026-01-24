package timesheet

import (
	"net/http"
	"strconv"

	"axiapac.com/axiapac/oktedi/model"
	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

func (ep *Endpoint) Get(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse("Invalid id"))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	var ts model.OktediTimesheet
	if err := db.Preload("Employee").
		Preload("Project").
		Preload("CostCentre").
		First(&ts, id).Error; err != nil {
		c.JSON(http.StatusNotFound, web.NewErrorResponse("Timesheet not found"))
		return
	}

	dateStr := ts.Date.Format("2006-01-02")

	// Fetch Clockin Records
	var clockinRecords []model.ClockinRecord
	if ts.Employee.IdentificationTag != "" {
		if err := db.Where("tag = ? AND date = ?", ts.Employee.IdentificationTag, dateStr).
			Find(&clockinRecords).Error; err != nil {
			c.JSON(http.StatusInternalServerError, web.NewErrorResponse("Failed to fetch clockin records"))
			return
		}
	}

	// Fetch Supervisor Records
	var supervisorRecords []model.SupervisorRecord
	if err := db.Where("employee_id = ? AND date = ?", ts.EmployeeID, dateStr).
		Find(&supervisorRecords).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse("Failed to fetch supervisor records"))
		return
	}

	// Map to DTO
	dto := OktediTimesheetDTO{
		ID:           ts.ID,
		Date:         ts.Date,
		Hours:        ts.Hours,
		StartTime:    ts.StartTime,
		FinishTime:   ts.FinishTime,
		ReviewStatus: ts.ReviewStatus,
		Approved:     ts.Approved,
		TimesheetID:  ts.TimesheetID,
		Employee: EmployeeDTO{
			ID:        ts.Employee.EmployeeID,
			Code:      ts.Employee.Code,
			FirstName: ts.Employee.FirstName,
			Surname:   ts.Employee.Surname,
		},
	}

	if ts.Project.JobID != 0 {
		dto.Job = JobDTO{
			ID:          ts.Project.JobID,
			JobNo:       ts.Project.JobNo,
			Description: ts.Project.Description,
		}
	}

	if ts.CostCentre.CostCentreID != 0 {
		dto.CostCentre = CostCentreDTO{
			ID:          ts.CostCentre.CostCentreID,
			Code:        ts.CostCentre.Code,
			Description: ts.CostCentre.Description,
		}
	}

	clockinDTOs := make([]ClockinRecordDTO, len(clockinRecords))
	for i, r := range clockinRecords {
		clockinDTOs[i] = ClockinRecordDTO{
			ID:        r.ID,
			Tag:       r.Tag,
			Date:      r.Date,
			Kind:      r.Kind,
			Timestamp: r.Timestamp,
			DeviceID:  r.DeviceID,
		}
	}

	supervisorDTOs := make([]SupervisorRecordDTO, len(supervisorRecords))
	for i, r := range supervisorRecords {
		supervisorDTOs[i] = SupervisorRecordDTO{
			ID:           r.ID,
			SupervisorID: r.SupervisorId,
			Project:      r.Project,
			Wbs:          r.Wbs,
			Date:         r.Date,
			Clockin:      r.Clockin,
			Clockout:     r.Clockout,
			DeviceID:     r.DeviceID,
		}
	}

	res := OktediTimesheetDetailDTO{
		OktediTimesheet:   dto,
		ClockinRecords:    clockinDTOs,
		SupervisorRecords: supervisorDTOs,
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(res))
}
