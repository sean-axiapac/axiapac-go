package timesheet

import (
	"net/http"
	"strconv"

	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

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
	Logic   string   `json:"logic"` // "and" or "or"
	Filters []Filter `json:"filters"`
}

type SearchParams struct {
	StartDate   *web.DateOnly `json:"startDate" binding:"required"`
	EndDate     *web.DateOnly `json:"endDate" binding:"required"`
	Supervisors []int32       `json:"supervisors"`
	Projects    []int32       `json:"projects"`
	Employees   []int32       `json:"employees"`
	Sorts       []Sort        `json:"sorts"`
	Filters     *FilterGroup  `json:"filters"`
}

func (ep *Endpoint) Search(c *gin.Context) {
	var searchParams SearchParams

	// Parse JSON body
	if err := c.ShouldBindJSON(&searchParams); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
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
	timesheets, total, err := SearchTimesheets(db, searchParams, limit, offset)

	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSearchResponse(timesheets, total))
}
