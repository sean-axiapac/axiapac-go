package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	v1 "axiapac.com/axiapac/axiapac/v1"
	"axiapac.com/axiapac/axiapac/v1/common"
	"axiapac.com/axiapac/axiapac/v1/common/eraid"
	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/core/models"
	"axiapac.com/axiapac/oktedi/model"
	"axiapac.com/axiapac/security"
	"axiapac.com/axiapac/utils"
	"gorm.io/gorm"
)

var tz = time.FixedZone("AEST", 10*3600)

func CreateClient(user *models.User, domain string) (*v1.AxiapacClient, error) {
	secret := os.Getenv("AXIAPAC_SIGNING_SECRET")
	url := os.Getenv("AXIAPAC_URL")
	if domain == "localhost" {
		url = "http://localhost:8080"
	} else {
		url = fmt.Sprintf("https://%s", domain)
	}

	token, err := security.CreateIdentityToken(&security.AxiapacIdentity{
		Id:       int(user.ID),
		UserName: user.UserName,
		Provider: user.Provider,
		Email:    user.Email,
	}, secret, 3600)

	if err != nil {
		return nil, err
	}

	return v1.NewAxiapacClient(url, token), nil
}

func SyncOktediTimesheet(db *gorm.DB, client *v1.AxiapacClient, source *model.OktediTimesheet) error {
	if !source.Approved {
		return nil
	}

	// 1. Fetch reference data needed for conversion
	var emp models.Employee
	if err := db.First(&emp, source.EmployeeID).Error; err != nil {
		return fmt.Errorf("employee not found: %w", err)
	}

	var labourRate models.LabourRate
	if err := db.First(&labourRate, emp.LabourRateID).Error; err != nil {
		return fmt.Errorf("labour rate not found: %w", err)
	}

	var timeType models.PayrollTimeType
	if err := db.Where(&models.PayrollTimeType{Code: "ORD"}).First(&timeType).Error; err != nil {
		return fmt.Errorf("payroll time type ORD not found: %w", err)
	}

	// 2. Convert to Axiapac DTO
	rate, err := core.CalcEmployeeRate(db, &emp, &labourRate, &timeType)
	if err != nil {
		return fmt.Errorf("failed to calculate employee rate: %w", err)
	}

	date := source.Date
	hours := source.Hours

	dto := &v1.TimesheetDTO{
		ID:             0,
		EraId:          eraid.Draft,
		Employee:       common.IdCodeDTO{ID: emp.EmployeeID, Code: emp.Code},
		Date:           date.Format("2006-01-02"),
		PaidHours:      hours,
		WorkedHours:    utils.Ptr(hours),
		TimesheetItems: []v1.TimesheetItemDTO{},
	}

	item := &v1.TimesheetItemDTO{
		ID:              0,
		Cost:            rate * hours,
		Hours:           hours,
		ChargeHours:     hours,
		PayrollTimeType: &common.IdCodeDTO{Code: "ORD"},
		LabourRate:      &common.IdCodeDTO{Code: labourRate.Code},
	}

	// Default start time to 08:00 if not set
	// Default start time to 08:00 if not set
	start := time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, tz)
	if !source.StartTime.IsZero() {
		start = source.StartTime
	}

	// Calculate ORD finish based on duration
	ordFinish := start.Add(time.Duration(hours * float64(time.Hour)))

	item.StartTime = utils.Ptr(start.Format("15:04"))
	item.FinishTime = utils.Ptr(ordFinish.Format("15:04"))

	// Resolve Job
	var jobID int32
	if source.ProjectID != nil {
		jobID = *source.ProjectID
	} else if emp.JobID != 0 {
		jobID = emp.JobID
	}

	if jobID == 0 {
		return fmt.Errorf("job is required")
	}

	var job models.Job
	if err := db.First(&job, jobID).Error; err != nil {
		return fmt.Errorf("job not found: %w", err)
	}
	item.Job = &common.JobNoDTO{JobNo: job.JobNo}

	// Resolve Cost Centre
	var ccID int32
	if source.CostCentreID != nil {
		ccID = *source.CostCentreID
	} else if emp.CostCentreID != 0 {
		ccID = emp.CostCentreID
	}

	if ccID != 0 {
		var cc models.CostCentre
		if err := db.First(&cc, ccID).Error; err != nil {
			return fmt.Errorf("cost centre not found: %w", err)
		}
		item.CostCentre = &common.FullCodeDTO{FullCode: cc.Code}
	}

	dto.TimesheetItems = append(dto.TimesheetItems, *item)

	// Apply break logic
	applyBreak(dto, source.Break)

	// 3. Resolve existing timesheet ID
	if source.TimesheetID != nil {
		dto.ID = int(*source.TimesheetID)
	}

	existing := models.Timesheet{}
	err = db.Model(&models.Timesheet{}).
		Where("EmployeeId = ?", dto.Employee.ID).
		Where("Date = ?", dto.Date).
		First(&existing).Error

	if err == nil {
		if existing.EraID != int32(eraid.Draft) {
			return fmt.Errorf("timesheet already exists in Axiapac and is not draft (ID=%d)", existing.TimesheetID)
		}
		// Sync the ID if found even if source.TimesheetID was missing or different
		dto.ID = int(existing.TimesheetID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// 4. Save to Axiapac
	res, err := client.Timesheets.Save(dto, true)
	if err != nil {
		return err
	}

	if !res.Status {
		return fmt.Errorf("save failed: %v", res.Error)
	}

	// 5. Update link in Oktedi
	return db.Model(source).Update("timesheet_id", res.Data.ID).Error
}

func applyBreak(dto *v1.TimesheetDTO, breakMinutes *int32) {
	if breakMinutes == nil || *breakMinutes <= 0 {
		return
	}

	if len(dto.TimesheetItems) == 0 {
		return
	}

	lastItem := &dto.TimesheetItems[len(dto.TimesheetItems)-1]

	// Parse times
	start, err1 := time.Parse("15:04", *lastItem.FinishTime)

	if err1 != nil {
		return
	}

	breakHours := float64(*breakMinutes) / 60.0
	finish := start.Add(time.Duration(breakHours * float64(time.Hour)))

	breakItem := v1.TimesheetItemDTO{
		Cost:            0,
		Hours:           breakHours,
		ChargeHours:     breakHours,
		PayrollTimeType: &common.IdCodeDTO{Code: "ORD"},
		LabourRate:      &common.IdCodeDTO{Code: "BR"},
		Job:             nil,
		CostCentre:      nil,
		StartTime:       utils.Ptr(start.Format("15:04")),
		FinishTime:      utils.Ptr(finish.Format("15:04")),
	}

	dto.TimesheetItems = append(dto.TimesheetItems, breakItem)
}
