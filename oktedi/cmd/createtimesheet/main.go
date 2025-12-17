package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

//TODO: group records: records can be send by different devices, so we need to group them by employee and date

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
	group     *RecordGroup
	timesheet *v1.TimesheetDTO
	error     error
}

func ConvertRecords(db *gorm.DB, groups []*RecordGroup) ([]*Converted, error) {
	var converted []*Converted

	var employees []models.Employee
	if err := db.Model(&models.Employee{}).Find(&employees).Error; err != nil {
		return converted, err
	}

	var labourrates []models.LabourRate
	if err := db.Model(&models.LabourRate{}).Find(&labourrates).Error; err != nil {
		return converted, err
	}

	var timeType *models.PayrollTimeType
	if err := db.Model(&models.PayrollTimeType{}).Where(&models.PayrollTimeType{Code: "ORD"}).First(&timeType).Error; err != nil {
		return converted, fmt.Errorf("failed to find payroll time type ORD: %w", err)
	}

	for _, g := range groups {
		// fmt.Printf("record: %s %s %s\n", r.ID, r.Date, r.EmployeeID)

		dto, err := ConvertRecord(db, employees, labourrates, timeType, g)
		if err != nil {
			converted = append(converted, &Converted{
				group: g,
				error: err,
			})
		} else {
			converted = append(converted, &Converted{
				group:     g,
				timesheet: dto,
			})
		}
	}

	return converted, nil
}

func ConvertRecord(db *gorm.DB, employees []models.Employee, labourrates []models.LabourRate, timeType *models.PayrollTimeType, r *RecordGroup) (*v1.TimesheetDTO, error) {
	emp := utils.Find(employees, func(e models.Employee) bool {
		return strconv.FormatInt(int64(e.EmployeeID), 10) == r.EmployeeID
	})

	if emp == nil {
		return nil, fmt.Errorf("employee not found: %s", r.EmployeeID)
	}

	labourRate := utils.Find(labourrates, func(l models.LabourRate) bool {
		return l.LabourRateID == emp.LabourRateID
	})
	if labourRate == nil {
		return nil, fmt.Errorf("employee %s doesn't have a Usual Chargeout Rate defined in Axiapac", emp.Code)
	}

	rate, err := core.CalcEmployeeRate(db, emp, labourRate, timeType)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate employee rate: %w", err)
	}

	date, err := utils.ParseISOTime(r.Date)
	if err != nil {
		return nil, fmt.Errorf("failed to parse record date: %w", err)
	}
	start, err := utils.ParseISOTime(r.GetClockIn())
	if err != nil {
		return nil, fmt.Errorf("failed to parse clock-in time: %w", err)
	}
	finish, err := utils.ParseISOTime(r.GetClockOut())
	if err != nil {
		return nil, fmt.Errorf("failed to parse clock-out time: %w", err)
	}
	duration := finish.Sub(*start)
	hours := duration.Hours()

	dto := &v1.TimesheetDTO{
		ID:             0,
		EraId:          eraid.Draft,
		Employee:       common.IdCodeDTO{ID: emp.EmployeeID, Code: emp.Code},
		Date:           date.In(tz).Format("2006-01-02"),
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
		StartTime:       utils.Ptr(start.In(tz).Format("15:04")),
		FinishTime:      utils.Ptr(finish.In(tz).Format("15:04")),
	}

	if labourRate != nil {
		item.LabourRate = &common.IdCodeDTO{Code: labourRate.Code}
	}

	job := r.GetJob()
	if job != "" {
		item.Job = &common.JobNoDTO{JobNo: job}
	}
	subjob := r.GetSubjob()
	if subjob != "" {
		item.CostCentre = &common.FullCodeDTO{FullCode: subjob}
	}

	dto.TimesheetItems = append(dto.TimesheetItems, *item)

	return dto, nil
}

func ProcessRecords(db *gorm.DB, client *v1.AxiapacClient, groups []*RecordGroup) error {
	// convert records to timesheets
	fmt.Printf("converting %d record groups to timesheets\n", len(groups))
	converted, err := ConvertRecords(db, groups)
	if err != nil {
		return err
	}

	// save and update status
	total := len(converted)
	success := 0
	failed := 0
	for idx, data := range converted {
		recordIDs := utils.Map(data.group.Records, func(r *model.ClockinRecord) string { return r.ID })
		fmt.Printf("[RECORD] (%d/%d) %s %s %s ==========\n", idx+1, total, recordIDs, data.group.Date, data.group.EmployeeID)
		if data.error != nil {
			fmt.Printf("[ERROR] error converting record: %v\n", data.error)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "error"})
			failed++
			continue
		}
		ts := data.timesheet

		attached := models.Timesheet{}
		if err := db.Model(&models.Timesheet{}).Where(&models.Timesheet{EmployeeID: ts.Employee.ID}).
			Where("date = ?", ts.Date).
			Where("eraid!= ?", eraid.Deleted).
			First(&attached).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Printf("[ERROR] error fetching timesheet: %v\n", err)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "error"})
			failed++
			continue
		}

		if attached.TimesheetID != 0 && attached.EraID != int32(eraid.Draft) {
			fmt.Printf("[ERROR] timesheet already exists (EraId=%d) for employee %s on %s, skipping\n", attached.EraID, ts.Employee.Code, ts.Date)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "skipped"})
			failed++
			continue
		}
		ts.ID = int(attached.TimesheetID) // if draft exists, update it

		// r := data.record
		fmt.Printf("[SAVE] %s timesheet (%d): %s %s %.f %v %v %s %s\n", utils.FormatBoolean(ts.ID != 0, "update", "create"), ts.ID, ts.Employee.Code, ts.Date, ts.PaidHours,
			ts.TimesheetItems[0].StartTime, ts.TimesheetItems[0].FinishTime,
			utils.Format(ts.TimesheetItems[0].Job), ts.TimesheetItems[0].CostCentre)

		// save timesheet
		res, err := client.Timesheets.Save(ts)
		if err != nil {
			fmt.Printf("[ERROR] request failed: %v\n", err)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "error"})
			failed++
			continue
		}

		if res.Status {
			fmt.Printf("[SUCCESS] timesheet saved: timesheetId=%d\n", res.Data.ID)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "processed"})
			success++
		} else {
			fmt.Printf("[ERROR] save failed: %v\n", res.Error)
			db.Model(&model.ClockinRecord{}).
				Where("id IN ?", recordIDs).
				Updates(&model.ClockinRecord{ProcessStatus: "error"})
			failed++
		}
	}

	fmt.Printf("Done.\nTotal: %d, Success: %d, Failed: %d\n", total, success, failed)

	return nil
}

type RecordGroup struct {
	EmployeeID string
	Date       string
	Records    []*model.ClockinRecord
}

func (rg *RecordGroup) GetClockIn() string {
	if len(rg.Records) == 0 {
		return ""
	}
	utils.Filter(rg.Records, func(r *model.ClockinRecord) bool { return r.ClockIn != "" })
	// utils.Map(rg.Records)
	return rg.Records[0].ClockIn
}

func (rg *RecordGroup) GetClockOut() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[len(rg.Records)-1].ClockOut
}

func (rg *RecordGroup) GetJob() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[0].Job
}

func (rg *RecordGroup) GetSubjob() string {
	if len(rg.Records) == 0 {
		return ""
	}
	return rg.Records[0].Subjob
}

func PrepareRecords(records []*model.ClockinRecord) []*RecordGroup {
	// group by date
	var groups []*RecordGroup
	dategroups := utils.GroupBy(records, func(r *model.ClockinRecord) string { return r.Date })

	for date, recs := range dategroups {
		// group by employeeID
		employeegroups := utils.GroupBy(recs, func(r *model.ClockinRecord) string { return r.EmployeeID })
		for empID, r2 := range employeegroups {
			rg := &RecordGroup{
				EmployeeID: empID,
				Date:       date,
				Records:    r2,
			}
			groups = append(groups, rg)
		}
	}
	return groups
}

func Run(db *gorm.DB, client *v1.AxiapacClient, date time.Time) error {
	// get target records
	var records []*model.ClockinRecord
	if err := db.Model(&model.ClockinRecord{}).
		Where(&model.ClockinRecord{ProcessStatus: "pending"}).
		Where("date <= ?", date.Format("2006-01-02")).
		Find(&records).Error; err != nil {
		return err
	}

	// prepare records
	groups := PrepareRecords(records)

	return ProcessRecords(db, client, groups)
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

	yesterday := utils.BrisbaneNow().AddDate(0, 0, -1)
	if err := Run(db, client, yesterday); err != nil {
		fmt.Println("error:", err)
	}
}
