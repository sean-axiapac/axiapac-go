package employee

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/core/models"
	oktedicore "axiapac.com/axiapac/oktedi/core"
	common "axiapac.com/axiapac/oktedi/web/common"
	web "axiapac.com/axiapac/web/common"
	"axiapac.com/axiapac/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Endpoint struct {
	base common.Handler
}

func Register(r *gin.RouterGroup, dm *core.DatabaseManager) {
	endpoint := &Endpoint{base: common.Handler{Dm: dm}}
	r.POST("/employees/search", endpoint.Search)
	r.GET("/employees/:id", endpoint.Detail)
}

type Sort struct {
	Field string `json:"field"`
	Dir   string `json:"dir"`
}

type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type FilterGroup struct {
	Logic   string   `json:"logic"`
	Filters []Filter `json:"filters"`
}

type SearchParams struct {
	Sorts   []Sort       `json:"sorts"`
	Filters *FilterGroup `json:"filters"`
	// Status toggles. Rostered/NotRostered combine as an OR within the roster
	// dimension (both or neither = no roster filter); ProjectGap is independent.
	Rostered    bool `json:"rostered"`
	NotRostered bool `json:"notRostered"`
	ProjectGap  bool `json:"projectGap"`
	// Base toggles (default on in the UI): restrict to the active era and to
	// non-terminated employees. Toggle off to include other eras / terminated.
	ActiveEra     bool `json:"activeEra"`
	NotTerminated bool `json:"notTerminated"`
}

type EmployeeDTO struct {
	ID                    int32      `json:"id"`
	Code                  string     `json:"code"`
	FirstName             string     `json:"firstName" gorm:"column:first_name"`
	Surname               string     `json:"surname"`
	JobID                 int32      `json:"jobId" gorm:"column:job_id"`
	JobCode               string     `json:"jobCode" gorm:"column:job_code"`
	JobDescription        string     `json:"jobDescription" gorm:"column:job_description"`
	CostCentreID          int32      `json:"costCentreId" gorm:"column:cost_centre_id"`
	CostCentreCode        string     `json:"costCentreCode" gorm:"column:cost_centre_code"`
	CostCentreDescription string     `json:"costCentreDescription" gorm:"column:cost_centre_description"`
	RosterTimeTypeID      int32      `json:"rosterTimeTypeId" gorm:"column:roster_time_type_id"`
	RosterTimeType        string     `json:"rosterTimeType" gorm:"column:roster_time_type"`
	RosterStartDate       *time.Time `json:"rosterStartDate" gorm:"column:roster_start_date"`
}

// WorkHourDTO is one day's assigned work hours. DayOfWeek matches Go's
// time.Weekday() (0=Sunday..6=Saturday); Break is in minutes.
type WorkHourDTO struct {
	DayOfWeek int32  `json:"dayOfWeek"`
	Start     string `json:"start"`
	Finish    string `json:"finish"`
	Break     int32  `json:"break"`
}

// EmployeeDetailDTO is the single-employee view: the list row plus the assigned
// work hours (resolved from region or personal hours per UseCalendarWorkHours).
type EmployeeDetailDTO struct {
	EmployeeDTO
	UseCalendarWorkHours bool          `json:"useCalendarWorkHours"`
	WorkHours            []WorkHourDTO `json:"workHours"`
	// RosteredOn: whether the employee is rostered ON for the requested date
	// (roster-cycle calc; false for non-roster or off-cycle). RosterPeriodStart/
	// End: the current on-or-off stretch's date range ("2006-01-02"; "" when no
	// roster cycle). RosterPanel: the `rosterPanel` value from the Attributes
	// JSON ("" when absent/null).
	RosteredOn        bool   `json:"rosteredOn"`
	RosterPeriodStart string `json:"rosterPeriodStart"`
	RosterPeriodEnd   string `json:"rosterPeriodEnd"`
	RosterPanel       string `json:"rosterPanel"`
	// Active: current, non-terminated as of the requested date (same rule that
	// drives the dashboard's roster/absent gating). False = terminated / inactive.
	Active bool `json:"active"`
}

// An employee is "rostered" when both a roster time type and a roster start
// date are set. (Guard the date against the legacy zero/sentinel value.)
const rosteredCond = "(e.RosterPayrollTimeTypeId IS NOT NULL AND e.RosterPayrollTimeTypeId <> 0 AND e.RosterStartDate IS NOT NULL AND e.RosterStartDate > '1900-01-01')"

func (ep *Endpoint) Search(c *gin.Context) {
	var params SearchParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
		return
	}

	limit := 1000
	offset := 0
	if v, err := strconv.Atoi(c.Query("limit")); err == nil {
		limit = v
	}
	if v, err := strconv.Atoi(c.Query("offset")); err == nil {
		offset = v
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	query := db.Table("Employees e").
		Joins("LEFT JOIN jobs j ON j.jobid = e.JobId").
		Joins("LEFT JOIN costcentres cc ON cc.costcentreid = e.CostCentreId").
		Joins("LEFT JOIN PayrollTimeTypes tt ON tt.PayrollTimeTypeId = e.RosterPayrollTimeTypeId")

	if params.ActiveEra {
		query = query.Where("e.EraId = ?", 1)
	}
	if params.NotTerminated {
		// No end date (NULL / legacy sentinel) or one that hasn't passed yet.
		query = query.Where("(e.EndDate IS NULL OR e.EndDate <= '1900-01-01' OR e.EndDate >= CURDATE())")
	}

	// Roster toggle: apply only when exactly one of the two is set (both or
	// neither means "don't filter by roster status").
	if params.Rostered != params.NotRostered {
		if params.Rostered {
			query = query.Where(rosteredCond)
		} else {
			query = query.Where("NOT " + rosteredCond)
		}
	}
	if params.ProjectGap {
		query = query.Where("(e.JobId IS NULL OR e.JobId = 0)")
	}

	// Column text filters.
	fieldMap := map[string]string{
		"code":           "e.Code",
		"firstName":      "e.FirstName",
		"surname":        "e.Surname",
		"jobCode":        "j.JobNo",
		"costCentreCode": "cc.Code",
		"rosterTimeType": "tt.Description",
	}
	if params.Filters != nil {
		for _, f := range params.Filters.Filters {
			dbField, ok := fieldMap[f.Field]
			if !ok {
				continue
			}
			switch strings.ToLower(f.Operator) {
			case "contains":
				query = query.Where(fmt.Sprintf("%s LIKE ?", dbField), fmt.Sprintf("%%%v%%", f.Value))
			case "eq":
				query = query.Where(fmt.Sprintf("%s = ?", dbField), f.Value)
			}
		}
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	query = query.Select(`e.EmployeeId as id, e.Code as code, e.FirstName as first_name, e.Surname as surname,
		e.JobId as job_id, j.JobNo as job_code, j.Description as job_description,
		e.CostCentreId as cost_centre_id, cc.Code as cost_centre_code, cc.Description as cost_centre_description,
		e.RosterPayrollTimeTypeId as roster_time_type_id, tt.Description as roster_time_type,
		e.RosterStartDate as roster_start_date`)

	sortFieldMap := map[string]string{
		"code":            "e.Code",
		"firstName":       "e.FirstName",
		"surname":         "e.Surname",
		"jobCode":         "j.JobNo",
		"costCentreCode":  "cc.Code",
		"rosterTimeType":  "tt.Description",
		"rosterStartDate": "e.RosterStartDate",
	}
	for _, s := range params.Sorts {
		dbField, ok := sortFieldMap[s.Field]
		if !ok {
			continue
		}
		dir := "ASC"
		if s.Dir == "desc" {
			dir = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", dbField, dir))
	}
	if len(params.Sorts) == 0 {
		query = query.Order("e.Code ASC")
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var results []EmployeeDTO
	if err := query.Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSearchResponse(results, total))
}

// Detail returns one employee (same joined row as Search) plus the assigned
// work hours, for the HRM Employees info dialog.
func (ep *Endpoint) Detail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse("invalid employee id"))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	var dto EmployeeDTO
	err = db.Table("Employees e").
		Joins("LEFT JOIN jobs j ON j.jobid = e.JobId").
		Joins("LEFT JOIN costcentres cc ON cc.costcentreid = e.CostCentreId").
		Joins("LEFT JOIN PayrollTimeTypes tt ON tt.PayrollTimeTypeId = e.RosterPayrollTimeTypeId").
		Select(`e.EmployeeId as id, e.Code as code, e.FirstName as first_name, e.Surname as surname,
			e.JobId as job_id, j.JobNo as job_code, j.Description as job_description,
			e.CostCentreId as cost_centre_id, cc.Code as cost_centre_code, cc.Description as cost_centre_description,
			e.RosterPayrollTimeTypeId as roster_time_type_id, tt.Description as roster_time_type,
			e.RosterStartDate as roster_start_date`).
		Where("e.EmployeeId = ?", id).
		Take(&dto).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, web.NewErrorResponse("employee not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// The joined row doesn't carry the work-hours source flags, so load the
	// model to resolve region vs personal hours.
	var emp models.Employee
	if err := db.Where("EmployeeId = ?", id).Take(&emp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// Roster ON/OFF for the requested date (default today, optional ?date=).
	date := utils.BrisbaneNow()
	if q := c.Query("date"); q != "" {
		if parsed, perr := time.ParseInLocation("2006-01-02", q, time.UTC); perr == nil {
			date = parsed
		}
	}
	var timeType *models.PayrollTimeType
	if emp.RosterPayrollTimeTypeID != 0 {
		var tt models.PayrollTimeType
		if err := db.Where("PayrollTimeTypeId = ?", emp.RosterPayrollTimeTypeID).Take(&tt).Error; err == nil {
			timeType = &tt
		}
	}
	isRoster, valid, _ := oktedicore.ValidateRoster(emp, timeType)
	rosteredOn := isRoster && valid && oktedicore.IsRosteredOn(emp, timeType, date)
	periodStart, periodEnd := "", ""
	if valid {
		if ps, pe, ok := oktedicore.CurrentRosterPeriod(emp, timeType, date); ok {
			periodStart = ps.Format("2006-01-02")
			periodEnd = pe.Format("2006-01-02")
		}
	}

	rosterPanel := oktedicore.RosterPanel(emp)

	c.JSON(http.StatusOK, web.NewSuccessResponse(EmployeeDetailDTO{
		EmployeeDTO:          dto,
		UseCalendarWorkHours: emp.UseCalendarWorkHours,
		WorkHours:            loadAssignedWorkHours(db, emp),
		RosteredOn:           rosteredOn,
		RosterPeriodStart:    periodStart,
		RosterPeriodEnd:      periodEnd,
		RosterPanel:          rosterPanel,
		Active:               oktedicore.ActiveEmployee(emp, date),
	}))
}

// loadAssignedWorkHours returns the employee's assigned work hours for each
// configured day (0=Sunday..6=Saturday, ordered). It mirrors GetDefinedWorkHours:
// region/calendar hours when UseCalendarWorkHours is set, otherwise the
// employee's personal hours.
func loadAssignedWorkHours(db *gorm.DB, emp models.Employee) []WorkHourDTO {
	out := make([]WorkHourDTO, 0, 7)

	if emp.UseCalendarWorkHours {
		if emp.CalendarRegionID == 0 {
			return out
		}
		var rows []models.RegionWorkHour
		db.Where("CalendarRegionId = ?", emp.CalendarRegionID).Find(&rows)
		byDay := make(map[int32]models.RegionWorkHour, len(rows))
		for _, r := range rows {
			byDay[r.DayOfWeek] = r
		}
		for d := int32(0); d < 7; d++ {
			if wh, ok := byDay[d]; ok {
				out = append(out, WorkHourDTO{DayOfWeek: d, Start: wh.Start, Finish: wh.Finish, Break: wh.Break})
			}
		}
		return out
	}

	var rows []models.EmployeeWorkHour
	db.Where("EmployeeId = ?", emp.EmployeeID).Find(&rows)
	byDay := make(map[int32]models.EmployeeWorkHour, len(rows))
	for _, r := range rows {
		byDay[r.DayOfWeek] = r
	}
	for d := int32(0); d < 7; d++ {
		if wh, ok := byDay[d]; ok {
			out = append(out, WorkHourDTO{DayOfWeek: d, Start: wh.Start, Finish: wh.Finish, Break: wh.Break})
		}
	}
	return out
}
