package timesheet

import (
	"encoding/json"
	"fmt"
	"strings"
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
	ID           int32         `json:"id"`
	Code         string        `json:"code"`
	FirstName    string        `json:"firstName"`
	Surname      string        `json:"surname"`
	JobID        int32         `json:"jobId"`
	CostCentreID int32         `json:"costCentreId"`
	Job          JobDTO        `json:"job" gorm:"embedded;embeddedPrefix:job_"`
	CostCentre   CostCentreDTO `json:"costCentre" gorm:"embedded;embeddedPrefix:cost_centre_"`
}

type JobDTO struct {
	ID          int32  `json:"id"`
	JobNo       string `json:"jobNo"`
	Description string `json:"description"`
}

type CostCentreDTO struct {
	ID          int32  `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

type OktediTimesheetDTO2 struct {
	ID           int32         `json:"id"`
	Date         time.Time     `json:"date"`
	Hours        float64       `json:"hours"`
	ReviewStatus string        `json:"reviewStatus"`
	Approved     bool          `json:"approved"`
	Employee     EmployeeDTO   `json:"employee" gorm:"embedded;embeddedPrefix:employee_"`
	Job          JobDTO        `json:"project" gorm:"embedded;embeddedPrefix:project_"`
	CostCentre   CostCentreDTO `json:"costCentre" gorm:"embedded;embeddedPrefix:cost_centre_"`
}

func (dto OktediTimesheetDTO2) MarshalJSON() ([]byte, error) {
	type Alias OktediTimesheetDTO2
	return json.Marshal(&struct {
		Date string `json:"date"`
		*Alias
	}{
		Date:  dto.Date.Format("2006-01-02"),
		Alias: (*Alias)(&dto),
	})
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

func SearchTimesheets(db *gorm.DB, params SearchParams, limit, offset int) ([]OktediTimesheetDTO, int64, error) {
	var results []OktediTimesheetDTO

	query := db.Table("oktedi_timesheets t1").
		Select(`t1.*, e.EmployeeId, e.Code, e.FirstName, e.Surname, concat(j.jobNo, '/',cc.code) as assignments`).
		Joins("JOIN employees e ON e.employeeid = t1.employee_id").
		Joins("LEFT JOIN jobs j ON j.jobid = t1.project_id").
		Joins("LEFT JOIN costcentres cc ON cc.costcentreid = t1.cost_centre_id").
		Where("t1.date BETWEEN ? AND ?", params.StartDate.Time.Format("2006-01-02"), params.EndDate.Time.Format("2006-01-02"))

	if len(params.Projects) > 0 {
		query = query.Where("t1.project_id IN ?", params.Projects)
	}
	if len(params.Supervisors) > 0 {
		query = query.Where("e.reportstoid IN ?", params.Supervisors)
	}
	if len(params.Employees) > 0 {
		query = query.Where("t1.employee_id IN ?", params.Employees)
	}

	// Apply Filters
	fieldMap := map[string]string{
		"id":           "t1.id",
		"date":         "t1.date",
		"hours":        "t1.hours",
		"reviewStatus": "t1.review_status",
		"approved":     "t1.approved",
		"employeeCode": "e.Code",
		"firstName":    "e.FirstName",
		"surname":      "e.Surname",
		"employeeId":   "t1.employee_id",
		"projectId":    "t1.project_id",

		// UI custom fields
		"name":        "concat(e.FirstName, ' ', e.Surname)",
		"assignments": "concat(j.jobNo, '/',cc.code)",
	}

	// Apply Filters
	if params.Filters != nil && len(params.Filters.Filters) > 0 {
		logic := strings.ToLower(params.Filters.Logic)
		if logic != "and" && logic != "or" {
			logic = "and" // default to AND
		}

		var conditions []string
		var values []interface{}

		for _, f := range params.Filters.Filters {
			dbField, ok := fieldMap[f.Field]
			if !ok {
				continue
			}

			var condition string
			switch strings.ToLower(f.Operator) {
			case "eq":
				condition = fmt.Sprintf("%s = ?", dbField)
				values = append(values, f.Value)
			case "neq":
				condition = fmt.Sprintf("%s != ?", dbField)
				values = append(values, f.Value)
			case "gt":
				condition = fmt.Sprintf("%s > ?", dbField)
				values = append(values, f.Value)
			case "gte":
				condition = fmt.Sprintf("%s >= ?", dbField)
				values = append(values, f.Value)
			case "lt":
				condition = fmt.Sprintf("%s < ?", dbField)
				values = append(values, f.Value)
			case "lte":
				condition = fmt.Sprintf("%s <= ?", dbField)
				values = append(values, f.Value)
			case "contains":
				condition = fmt.Sprintf("%s LIKE ?", dbField)
				values = append(values, fmt.Sprintf("%%%v%%", f.Value))
			case "in":
				condition = fmt.Sprintf("%s IN ?", dbField)
				values = append(values, f.Value)
			default:
				continue
			}

			if condition != "" {
				conditions = append(conditions, condition)
			}
		}

		if len(conditions) > 0 {
			if logic == "or" {
				query = query.Where(strings.Join(conditions, " OR "), values...)
			} else {
				// For AND, we can apply each condition separately for better query optimization
				for i, condition := range conditions {
					query = query.Where(condition, values[i])
				}
			}
		}
	}

	// count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply Sorts
	for _, s := range params.Sorts {
		dbField, ok := fieldMap[s.Field]
		if !ok {
			continue
		}
		direction := "ASC"
		if s.Dir == "desc" {
			direction = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", dbField, direction))
	}

	if len(params.Sorts) == 0 {
		query = query.Order("t1.date DESC, e.Surname ASC")
	}

	query = query.Limit(limit).Offset(offset)

	err := query.Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
func SearchTimesheets2(db *gorm.DB, params SearchParams, limit, offset int) ([]OktediTimesheetDTO2, int64, error) {
	var results []OktediTimesheetDTO2

	query := db.Table("oktedi_timesheets t1").
		Select(`t1.*, 
            e.EmployeeId as employee_id, e.Code as employee_code, e.FirstName as employee_first_name, e.Surname as employee_surname, e.JobID as employee_job_id, e.CostCentreID as employee_cost_centre_id,
            ej.JobId as employee_job_id, ej.JobNo as employee_job_job_no, ej.Description as employee_job_description,
            ecc.CostCentreId as employee_cost_centre_id, ecc.Code as employee_cost_centre_code, ecc.Description as employee_cost_centre_description,
            j.JobId as job_id, j.JobNo as job_job_no, j.Description as job_description,
            cc.CostCentreId as cost_centre_id, cc.Code as cost_centre_code, cc.Description as cost_centre_description`).
		Joins("JOIN employees e ON e.employeeid = t1.employee_id").
		Joins("LEFT JOIN jobs ej ON ej.jobid = e.jobid").
		Joins("LEFT JOIN costcentres ecc ON ecc.costcentreid = e.costcentreid").
		Joins("LEFT JOIN jobs j ON j.jobid = t1.project_id").
		Joins("LEFT JOIN costcentres cc ON cc.costcentreid = t1.cost_centre_id").
		Where("t1.date BETWEEN ? AND ?", params.StartDate.Time.Format("2006-01-02"), params.EndDate.Time.Format("2006-01-02"))

	if len(params.Projects) > 0 {
		query = query.Where("t1.project_id IN ?", params.Projects)
	}
	if len(params.Supervisors) > 0 {
		query = query.Where("e.reportstoid IN ?", params.Supervisors)
	}
	if len(params.Employees) > 0 {
		query = query.Where("t1.employee_id IN ?", params.Employees)
	}

	// Apply Filters
	fieldMap := map[string]string{
		"id":           "t1.id",
		"date":         "t1.date",
		"hours":        "t1.hours",
		"reviewStatus": "t1.review_status",
		"approved":     "t1.approved",
		"employeeCode": "e.Code",
		"firstName":    "e.FirstName",
		"surname":      "e.Surname",
		"employeeId":   "t1.employee_id",
		"projectId":    "t1.project_id",

		// UI custom fields
		"name":        "concat(e.FirstName, ' ', e.Surname)",
		"assignments": "concat(j.jobNo, '/',cc.code)",
	}

	// Apply Filters
	if params.Filters != nil && len(params.Filters.Filters) > 0 {
		logic := strings.ToLower(params.Filters.Logic)
		if logic != "and" && logic != "or" {
			logic = "and" // default to AND
		}

		var conditions []string
		var values []interface{}

		for _, f := range params.Filters.Filters {
			dbField, ok := fieldMap[f.Field]
			if !ok {
				continue
			}

			var condition string
			switch strings.ToLower(f.Operator) {
			case "eq":
				condition = fmt.Sprintf("%s = ?", dbField)
				values = append(values, f.Value)
			case "neq":
				condition = fmt.Sprintf("%s != ?", dbField)
				values = append(values, f.Value)
			case "gt":
				condition = fmt.Sprintf("%s > ?", dbField)
				values = append(values, f.Value)
			case "gte":
				condition = fmt.Sprintf("%s >= ?", dbField)
				values = append(values, f.Value)
			case "lt":
				condition = fmt.Sprintf("%s < ?", dbField)
				values = append(values, f.Value)
			case "lte":
				condition = fmt.Sprintf("%s <= ?", dbField)
				values = append(values, f.Value)
			case "contains":
				condition = fmt.Sprintf("%s LIKE ?", dbField)
				values = append(values, fmt.Sprintf("%%%v%%", f.Value))
			case "in":
				condition = fmt.Sprintf("%s IN ?", dbField)
				values = append(values, f.Value)
			default:
				continue
			}

			if condition != "" {
				conditions = append(conditions, condition)
			}
		}

		if len(conditions) > 0 {
			if logic == "or" {
				query = query.Where(strings.Join(conditions, " OR "), values...)
			} else {
				// For AND, we can apply each condition separately for better query optimization
				for i, condition := range conditions {
					query = query.Where(condition, values[i])
				}
			}
		}
	}

	// count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply Sorts
	for _, s := range params.Sorts {
		dbField, ok := fieldMap[s.Field]
		if !ok {
			continue
		}
		direction := "ASC"
		if s.Dir == "desc" {
			direction = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", dbField, direction))
	}

	if len(params.Sorts) == 0 {
		query = query.Order("t1.date DESC, e.Surname ASC")
	}

	query = query.Limit(limit).Offset(offset)

	err := query.Find(&results).Error
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}
