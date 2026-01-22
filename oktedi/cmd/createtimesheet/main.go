package main

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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var tz = time.FixedZone("AEST", 10*3600)

func CreateClient(url string) (*v1.AxiapacClient, error) {
	secret := os.Getenv("AXIAPAC_SIGNING_SECRET")
	token, err := security.CreateIdentityToken(&security.AxiapacIdentity{
		Id:       5,
		UserName: "sean",
		Provider: "local",
		Email:    "sean.tang@axiapac.com.au",
	}, secret, 3600)

	if err != nil {
		return nil, err
	}

	// Create transport with test server base URL
	client := v1.NewAxiapacClient(url, token)

	return client, nil
}

type Converted struct {
	source    *model.OktediTimesheet
	timesheet *v1.TimesheetDTO
	error     error
}

func ConvertTimesheets(db *gorm.DB, sources []model.OktediTimesheet) ([]*Converted, error) {
	var converted []*Converted

	var employees []models.Employee
	if err := db.Model(&models.Employee{}).Find(&employees).Error; err != nil {
		return converted, err
	}
	empMap := make(map[int32]models.Employee)
	for _, e := range employees {
		empMap[e.EmployeeID] = e
	}

	var labourrates []models.LabourRate
	if err := db.Model(&models.LabourRate{}).Find(&labourrates).Error; err != nil {
		return converted, err
	}
	lrMap := make(map[int32]models.LabourRate) // LabourRateID -> LabourRate
	for _, l := range labourrates {
		lrMap[l.LabourRateID] = l
	}

	var jobs []models.Job
	if err := db.Model(&models.Job{}).Find(&jobs).Error; err != nil {
		return converted, err
	}
	jobMap := make(map[int32]models.Job)
	for _, j := range jobs {
		jobMap[j.JobID] = j
	}

	var costCentres []models.CostCentre
	if err := db.Model(&models.CostCentre{}).Find(&costCentres).Error; err != nil {
		return converted, err
	}
	ccMap := make(map[int32]models.CostCentre)
	for _, c := range costCentres {
		ccMap[c.CostCentreID] = c
	}

	var timeType *models.PayrollTimeType
	if err := db.Model(&models.PayrollTimeType{}).Where(&models.PayrollTimeType{Code: "ORD"}).First(&timeType).Error; err != nil {
		return converted, fmt.Errorf("failed to find payroll time type ORD: %w", err)
	}

	for i := range sources {
		source := &sources[i]
		dto, err := ConvertTimesheet(db, empMap, lrMap, jobMap, ccMap, timeType, source)
		if err != nil {
			converted = append(converted, &Converted{
				source: source,
				error:  err,
			})
		} else {
			converted = append(converted, &Converted{
				source:    source,
				timesheet: dto,
			})
		}
	}

	return converted, nil
}

func ConvertTimesheet(
	db *gorm.DB,
	empMap map[int32]models.Employee,
	lrMap map[int32]models.LabourRate,
	jobMap map[int32]models.Job,
	ccMap map[int32]models.CostCentre,
	timeType *models.PayrollTimeType,
	source *model.OktediTimesheet,
) (*v1.TimesheetDTO, error) {

	emp, ok := empMap[source.EmployeeID]
	if !ok {
		return nil, fmt.Errorf("employee not found: %d", source.EmployeeID)
	}

	labourRate, ok := lrMap[emp.LabourRateID]
	if !ok {
		return nil, fmt.Errorf("employee %s doesn't have a Labour Rate defined (id=%d)", emp.Code, emp.LabourRateID)
	}

	rate, err := core.CalcEmployeeRate(db, &emp, &labourRate, timeType)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate employee rate: %w", err)
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
		Description:     "",
		Cost:            rate * hours,
		Hours:           hours,
		ChargeHours:     hours,
		PayrollTimeType: &common.IdCodeDTO{Code: "ORD"},
	}

	// Default start time to 08:00 if not set
	start := time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, tz)
	if !source.StartTime.IsZero() {
		start = source.StartTime
	}

	finish := start.Add(time.Duration(hours * float64(time.Hour)))
	if !source.FinishTime.IsZero() {
		finish = source.FinishTime
	}

	item.StartTime = utils.Ptr(start.Format("15:04"))
	item.FinishTime = utils.Ptr(finish.Format("15:04"))

	item.LabourRate = &common.IdCodeDTO{Code: labourRate.Code}

	// Resolve Job (Required)
	var jobID int32
	if source.ProjectID != nil {
		jobID = *source.ProjectID
	} else if emp.JobID != 0 {
		jobID = emp.JobID
	}

	if jobID == 0 {
		return nil, fmt.Errorf("job is required (source project_id is nil and employee default job_id is 0)")
	}

	if job, ok := jobMap[jobID]; ok {
		item.Job = &common.JobNoDTO{JobNo: job.JobNo}
	} else {
		return nil, fmt.Errorf("job not found for ID: %d", jobID)
	}

	// Resolve Cost Centre (Optional, but with default)
	var ccID int32
	if source.CostCentreID != nil {
		ccID = *source.CostCentreID
	} else if emp.CostCentreID != 0 {
		ccID = emp.CostCentreID
	}

	if ccID != 0 {
		if cc, ok := ccMap[ccID]; ok {
			item.CostCentre = &common.FullCodeDTO{FullCode: cc.Code}
		}
	}

	dto.TimesheetItems = append(dto.TimesheetItems, *item)

	return dto, nil
}

func ProcessTimesheets(db *gorm.DB, client *v1.AxiapacClient, sources []model.OktediTimesheet) error {
	// convert records to timesheets
	fmt.Printf("converting %d oktedi timesheets\n", len(sources))
	converted, err := ConvertTimesheets(db, sources)
	if err != nil {
		return err
	}

	// save and update status
	total := len(converted)
	success := 0
	failed := 0

	for idx, data := range converted {
		source := data.source
		fmt.Printf("[TS] (%d/%d) ID=%d Date=%s EmpID=%d ==========\n", idx+1, total, source.ID, source.Date.Format("2006-01-02"), source.EmployeeID)

		if data.error != nil {
			fmt.Printf("[ERROR] error converting: %v\n", data.error)
			failed++
			continue
		}
		ts := data.timesheet

		// Check if timesheet already exists
		existing := models.Timesheet{}
		err := db.Model(&models.Timesheet{}).
			Where("EmployeeId = ?", ts.Employee.ID).
			Where("Date = ?", ts.Date).
			First(&existing).Error

		if err == nil {
			// Found existing timesheet
			if existing.EraID == int32(eraid.Draft) {
				// It's a draft, so we can update it
				ts.ID = int(existing.TimesheetID)
				fmt.Printf("[UPDATE] updating existing DRAFT timesheet ID=%d\n", ts.ID)
			} else {
				// It's not a draft, error out
				fmt.Printf("[ERROR] timesheet already exists and is not draft (ID=%d, EraID=%d)\n", existing.TimesheetID, existing.EraID)
				failed++
				continue
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			// DB Error
			fmt.Printf("[ERROR] database error checking existing timesheet: %v\n", err)
			failed++
			continue
		}

		// Proceed to Save (Create or Update)
		fmt.Printf("[SAVE] %s timesheet: %s %s %.2f\n",
			utils.FormatBoolean(ts.ID != 0, "update", "create"),
			ts.Employee.Code, ts.Date, ts.PaidHours)

		// save timesheet
		res, err := client.Timesheets.Save(ts, true)
		if err != nil {
			fmt.Printf("[ERROR] request failed: %v\n", err)
			failed++
			continue
		}

		if res.Status {
			fmt.Printf("[SUCCESS] timesheet saved: timesheetId=%d\n", res.Data.ID)
			// Update source with new ID
			if err := db.Model(&model.OktediTimesheet{}).
				Where("id = ?", source.ID).
				Updates(map[string]interface{}{"timesheet_id": res.Data.ID}).Error; err != nil {
				fmt.Printf("[ERROR] failed to update source link: %v\n", err)
			}
			success++
		} else {
			fmt.Printf("[ERROR] save failed: %v\n", res.Error)
			failed++
		}
	}

	fmt.Printf("Done.\nTotal: %d, Success: %d, Failed: %d\n", total, success, failed)

	return nil
}

func Run(db *gorm.DB, client *v1.AxiapacClient, date *time.Time) error {
	// get approved timesheets not yet synced
	var sources []model.OktediTimesheet
	query := db.Model(&model.OktediTimesheet{}).
		Where("approved = ?", true) //.Where("timesheet_id IS NULL")

	if date != nil {
		query = query.Where("date = ?", date.Format("2006-01-02"))
	}

	if err := query.Find(&sources).Error; err != nil {
		return err
	}

	return ProcessTimesheets(db, client, sources)
}

func main() {

	dsn := "root:development@tcp(localhost:3306)/oktedi?parseTime=true"
	db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	client, err := CreateClient("http://localhost:8080")
	if err != nil {
		fmt.Println("error creating client:", err)
		return
	}

	// Optional: pass date as arg or filter
	// For now, process all pending approved
	if err := Run(db, client, utils.Ptr(utils.MustParseDate("2025-12-17"))); err != nil {
		fmt.Println("error:", err)
	}
}
