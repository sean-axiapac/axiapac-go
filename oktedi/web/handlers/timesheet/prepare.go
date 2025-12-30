package timesheet

import (
	"net/http"

	oktedi "axiapac.com/axiapac/oktedi/core"
	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

type PrepareParams struct {
	StartDate   *web.DateOnly `json:"startDate" binding:"required"`
	EndDate     *web.DateOnly `json:"endDate" binding:"required"`
	Supervisors []int32       `json:"supervisors"`
	Employees   []int32       `json:"employees"`
}

func (ep *Endpoint) Prepare(c *gin.Context) {
	var params PrepareParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	opts := oktedi.PrepareOptions{
		StartDate:   params.StartDate.Time,
		EndDate:     params.EndDate.Time,
		Supervisors: params.Supervisors,
		Employees:   params.Employees,
	}

	if err := oktedi.Prepare(db, opts); err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{}))
}
