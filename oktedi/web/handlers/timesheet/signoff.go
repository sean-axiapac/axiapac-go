package timesheet

import (
	"net/http"

	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

func (ep *Endpoint) SignOff(c *gin.Context) {
	var searchParams SearchParams

	// Parse JSON body
	if err := c.ShouldBindJSON(&searchParams); err != nil {
		c.JSON(http.StatusBadRequest, web.NewErrorResponse(web.FormatBindingError(err)))
		return
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	// Build the search query
	query := BuildSearchQuery(db, searchParams)

	// Update only those that are approved
	query = query.Where("t1.approved = ?", true)

	// Fetch IDs first to avoid issue with joins in mass update
	var ids []int32
	if err := query.Pluck("t1.id", &ids).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	if len(ids) == 0 {
		c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{
			"updated": 0,
		}))
		return
	}

	// Perform the update by ID list
	res := db.Table("oktedi_timesheets").Where("id IN ?", ids).UpdateColumn("review_status", "signed-off")
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(res.Error.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{
		"updated": res.RowsAffected,
	}))
}
