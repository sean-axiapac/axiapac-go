package timesheet

import (
	"net/http"
	"strconv"
	"time"

	"axiapac.com/axiapac/core"
	oktedi "axiapac.com/axiapac/oktedi/core"
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
	r.PUT("/timesheets/:id", endpoint.Update)
	// r.GET("/owner-disbursments/:id", endpoint.Find)
	// r.GET("/owner-disbursments/:id/statements", endpoint.ListStatements)

	// convert records to oktedi timesheets
	r.POST("/timesheets", endpoint.ProcessClockInRecords)
	r.POST("/timesheets/prepare", endpoint.Prepare)
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
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
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

type ClockInRecordsProcessParamsDTO struct {
	Date *string `json:"date,omitempty"`
}

func (ep *Endpoint) ProcessClockInRecords(c *gin.Context) {
	// get date from body
	var params ClockInRecordsProcessParamsDTO
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
		return
	}
	// convert params.Date to time.Date
	var date *time.Time
	if params.Date != nil {
		todate, err := time.Parse("2006-01-02", *params.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, web.NewErrorResponse(err.Error()))
			return
		}
		date = &todate
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	if err := oktedi.ProcessClockInRecords(db, *date); err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{}))
}
