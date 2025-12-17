package timesheet

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type OktediTimesheetDTO struct {
	ID           int32       `json:"id"`
	Date         time.Time   `json:"date"`
	Hours        float64     `json:"hours"`
	Assignments  string      `json:"assignments"`
	ReviewStatus string      `json:"reviewStatus"`
	Approved     bool        `json:"approved"`
	Employee     EmployeeDTO `gorm:"embedded"`
}

type EmployeeDTO struct {
	ID        int32  `json:"id"`
	Code      string `json:"code"`
	FirstName string `json:"firstName"`
	Surname   string `json:"surname"`
}

func (dto OktediTimesheetDTO) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID           int32       `json:"id"`
		ReviewStatus string      `json:"reviewStatus"`
		Date         string      `json:"date"`
		Hours        float64     `json:"hours"`
		Assignments  string      `json:"assignments"`
		Approved     bool        `json:"approved"`
		Employee     EmployeeDTO `json:"employee"`
	}{
		ID:           dto.ID,
		ReviewStatus: dto.ReviewStatus,
		Date:         dto.Date.Format("2006-01-02"),
		Hours:        dto.Hours,
		Assignments:  dto.Assignments,
		Approved:     dto.Approved,
		Employee:     dto.Employee,
	})
}

func SearchTimesheets(db *gorm.DB, startDate, endDate string, supervisors, projects, employees []int32, limit, offset int) ([]OktediTimesheetDTO, int64, error) {
	var results []OktediTimesheetDTO

	query := db.Table("oktedi_timesheets t1").
		Select(`t1.*,
	        e.*`).
		Joins("LEFT OUTER JOIN timesheets t ON t.timesheetid = t1.id").
		Joins("JOIN employees e ON e.employeeid = t1.employee_id").
		Where("t1.date BETWEEN ? AND ?", startDate, endDate)

	if len(projects) > 0 {
		query = query.Where("t1.project_id IN ?", projects)
	}
	if len(supervisors) > 0 {
		query = query.Where("e.reportstoid IN ?", supervisors)
	}
	if len(employees) > 0 {
		query = query.Where("t1.employee_id IN ?", employees)
	}

	// count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = query.Limit(limit).Offset(offset)

	err := query.Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
