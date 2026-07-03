package whoami

import (
	"net/http"
	"strconv"
	"strings"

	"axiapac.com/axiapac/core"
	common "axiapac.com/axiapac/oktedi/web/common"
	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Endpoint struct {
	base common.Handler
}

func Register(r *gin.RouterGroup, dm *core.DatabaseManager) {
	endpoint := &Endpoint{base: common.Handler{Dm: dm}}
	r.GET("/identity", endpoint.Identity)
}

// IdentityDTO is the current user's oktedi profile, derived from the JWT (no
// caller-supplied identifier — so it's self-scoped, not an IDOR). The employee
// section is populated when the user is linked to an employee.
type IdentityDTO struct {
	UserID    int32  `json:"userId"`
	IsAdmin   bool   `json:"isAdmin"`
	FirstName string `json:"firstName"`
	Surname   string `json:"surname"`
	Email     string `json:"email"`

	EmployeeID     int32  `json:"employeeId"`
	Code           string `json:"code"`
	Picture        string `json:"picture"`
	JobCode        string `json:"jobCode"`
	JobName        string `json:"jobName"`
	CostCentreCode string `json:"costCentreCode"`
	CostCentreName string `json:"costCentreName"`
	SupervisorName string `json:"supervisorName"`
	BackToBackName string `json:"backToBackName"`

	// Resolved default dashboard project (supervisor's job; 0 when none).
	IsSupervisor bool   `json:"isSupervisor"`
	ProjectID    int32  `json:"projectId"`
	ProjectCode  string `json:"projectCode"`
	ProjectName  string `json:"projectName"`
}

// jwtUserID pulls the authenticated user's id from the `nameid` claim, which the
// auth middleware attaches. Handles both string and numeric JSON encodings.
func jwtUserID(c *gin.Context) (int32, bool) {
	v, ok := c.Get("claims")
	if !ok {
		return 0, false
	}
	claims, ok := v.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	switch id := claims["nameid"].(type) {
	case string:
		n, err := strconv.Atoi(id)
		if err != nil {
			return 0, false
		}
		return int32(n), true
	case float64:
		return int32(id), true
	}
	return 0, false
}

// Identity returns the current user's profile, resolved from the JWT:
// user → Users.EmployeeId → employee. The default project is the user's own job
// when they're a supervisor (have direct reports), else their back-to-back
// partner's job when that partner is a supervisor, else none. Own supervisor
// identity wins when both apply.
func (ep *Endpoint) Identity(c *gin.Context) {
	userID, ok := jwtUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, web.NewErrorResponse("no authenticated user"))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	var user struct {
		EmployeeID int32
		SysAdmin   bool
		FirstName  string
		LastName   string
		Email      string
	}
	if err := db.Table("users").
		Select("EmployeeId as employee_id, SysAdmin as sys_admin, FirstName as first_name, LastName as last_name, Email as email").
		Where("Id = ?", userID).
		Take(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	dto := IdentityDTO{
		UserID:    userID,
		IsAdmin:   user.SysAdmin,
		FirstName: user.FirstName,
		Surname:   user.LastName,
		Email:     user.Email,
	}

	if user.EmployeeID == 0 {
		// Admin/user without a linked employee — return the user-level identity.
		c.JSON(http.StatusOK, web.NewSuccessResponse(dto))
		return
	}

	var me struct {
		ID             int32
		FirstName      string
		Surname        string
		Code           string
		Picture        string
		JobID          int32
		JobCode        string
		JobName        string
		CostCentreCode string
		CostCentreName string
		ReportsToID    int32
		BackToBackID   int32
	}
	if err := db.Table("employees e").
		Joins("LEFT JOIN jobs j ON j.jobid = e.jobid").
		Joins("LEFT JOIN costcentres cc ON cc.costcentreid = e.costcentreid").
		Select(`e.employeeid as id, e.firstname as first_name, e.surname as surname, e.code as code, e.picture as picture,
			e.jobid as job_id, j.jobno as job_code, j.description as job_name,
			cc.code as cost_centre_code, cc.description as cost_centre_name,
			e.reportstoid as reports_to_id,
			CAST(JSON_EXTRACT(e.attributes, '$.backToBack.id') AS UNSIGNED) as back_to_back_id`).
		Where("e.employeeid = ?", user.EmployeeID).
		Take(&me).Error; err != nil {
		// Linked employee row missing — still return the user-level identity.
		c.JSON(http.StatusOK, web.NewSuccessResponse(dto))
		return
	}

	hasReports := func(id int32) bool {
		if id == 0 {
			return false
		}
		var n int64
		db.Table("employees").Where("reportstoid = ?", id).Count(&n)
		return n > 0
	}
	nameOf := func(id int32) string {
		if id == 0 {
			return ""
		}
		var n struct {
			FirstName string
			Surname   string
		}
		db.Table("employees").Select("firstname as first_name, surname as surname").Where("employeeid = ?", id).Take(&n)
		return strings.TrimSpace(n.FirstName + " " + n.Surname)
	}

	isSupervisor := hasReports(me.ID)
	projectJobID := int32(0)
	if isSupervisor {
		projectJobID = me.JobID // own supervisor identity
	} else if hasReports(me.BackToBackID) {
		db.Table("employees").Select("jobid").Where("employeeid = ?", me.BackToBackID).Scan(&projectJobID)
	}

	dto.FirstName = me.FirstName
	dto.Surname = me.Surname
	dto.EmployeeID = me.ID
	dto.Code = me.Code
	dto.Picture = me.Picture
	dto.JobCode = me.JobCode
	dto.JobName = me.JobName
	dto.CostCentreCode = me.CostCentreCode
	dto.CostCentreName = me.CostCentreName
	dto.SupervisorName = nameOf(me.ReportsToID)
	dto.BackToBackName = nameOf(me.BackToBackID)
	dto.IsSupervisor = isSupervisor

	if projectJobID != 0 {
		var job struct {
			ID   int32
			Code string
			Name string
		}
		db.Table("jobs").Select("jobid as id, jobno as code, description as name").Where("jobid = ?", projectJobID).Scan(&job)
		dto.ProjectID = job.ID
		dto.ProjectCode = job.Code
		dto.ProjectName = job.Name
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(dto))
}
