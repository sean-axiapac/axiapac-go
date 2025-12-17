package timesheet

import (
	"net/http"
	"strconv"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/oktedi/model"
	common "axiapac.com/axiapac/oktedi/web/common"
	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

type Endpoint struct {
	base common.Handler
}

func Register(r *gin.RouterGroup, dm *core.DatabaseManager) {
	endpoint := &Endpoint{base: common.Handler{Dm: dm}}
	r.POST("/timesheets/search", endpoint.Search)
	// r.GET("/owner-disbursments/:id", endpoint.Find)
	// r.GET("/owner-disbursments/:id/statements", endpoint.ListStatements)

	r.PUT("/timesheets/:id", endpoint.Update)
}

type SearchParams struct {
	StartDate   *web.DateOnly `json:"startDate" binding:"required"`
	EndDate     *web.DateOnly `json:"endDate" binding:"required"`
	Supervisors []int32       `json:"supervisors"`
	Projects    []int32       `json:"projects"`
	Employees   []int32       `json:"employees"`
}

func (ep *Endpoint) Search(c *gin.Context) {
	var searchParams SearchParams

	// Parse JSON body
	if err := c.ShouldBindJSON(&searchParams); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(err.Error()))
		return
	}

	// get limit, offset from query params
	limit := 1000
	offset := 0
	if val, err := strconv.Atoi(c.Query("limit")); err == nil {
		limit = val
	}
	if val, err := strconv.Atoi(c.Query("offset")); err == nil {
		offset = val
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	// timesheets, err := GetTimesheets(db, searchParams.StartDate.Time, searchParams.EndDate.Time, searchParams.Supervisors, searchParams.Projects, searchParams.Employees)
	timesheets, total, err := SearchTimesheets(db, searchParams.StartDate.Time.Format("2006-01-02"), searchParams.EndDate.Time.Format("2006-01-02"), searchParams.Supervisors, searchParams.Projects, searchParams.Employees,
		limit, offset)

	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSearchResponse(timesheets, total))
}

type OktediTimesheetUpdateDTO struct {
	Hours        *float64 `json:"hours,omitempty"`
	ReviewStatus *string  `json:"reviewStatus,omitempty"`
	Approved     *bool    `json:"approved,omitempty"`
}

func (ep *Endpoint) Update(c *gin.Context) {
	// get id from path
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse("Invalid id"))
		return
	}

	var updateDTO OktediTimesheetUpdateDTO
	if err := c.ShouldBindJSON(&updateDTO); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(err.Error()))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	// Update the timesheet in the database
	if err := db.Model(&model.OktediTimesheet{}).Where("id = ?", id).Updates(updateDTO).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{}))
}
