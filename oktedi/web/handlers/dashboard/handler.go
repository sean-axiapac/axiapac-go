package dashboard

import (
	"net/http"
	"time"

	"axiapac.com/axiapac/core"
	oktedi "axiapac.com/axiapac/oktedi/core"
	common "axiapac.com/axiapac/oktedi/web/common"
	"axiapac.com/axiapac/utils"
	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
)

type Endpoint struct {
	base common.Handler
}

func Register(r *gin.RouterGroup, dm *core.DatabaseManager) {
	endpoint := &Endpoint{base: common.Handler{Dm: dm}}
	r.GET("/dashboard/attendance", endpoint.Attendance)
}

// Attendance returns the full per-employee attendance view for a date. The
// client holds this list and derives all metric cards, search and pagination
// from it, so this is the single read the dashboard polls.
//
//	GET /dashboard/attendance?date=YYYY-MM-DD  (date optional, defaults to today)
func (ep *Endpoint) Attendance(c *gin.Context) {
	now := utils.BrisbaneNow()

	date := now
	if q := c.Query("date"); q != "" {
		parsed, err := time.ParseInLocation("2006-01-02", q, time.UTC)
		if err != nil {
			c.JSON(http.StatusBadRequest, web.NewErrorResponse("invalid date; expected YYYY-MM-DD"))
			return
		}
		date = parsed
	}

	db, conn, err := ep.base.GetDB(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	defer conn.Close()

	result, err := oktedi.LoadAttendance(db, date, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(result))
}
