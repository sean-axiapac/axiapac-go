package dashboard

import (
	"net/http"
	"strconv"
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
	r.GET("/dashboard/evacuation-register", endpoint.EvacuationRegister)
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

// EvacuationRegister streams the Major Projects Attendance/ Evacuation Register
// as a PDF for a date, optionally scoped to a single project. It reuses the
// attendance read, then applies the register's own rules (clocked-in-only at a
// mapped device, area from device) inside EvacuationRegisterPDF.
//
//	GET /dashboard/evacuation-register?date=YYYY-MM-DD&projectId=N  (both optional)
func (ep *Endpoint) EvacuationRegister(c *gin.Context) {
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

	// Project scoping mirrors the dashboard's client-side filter (all projects
	// when absent).
	rows := result.Rows
	if q := c.Query("projectId"); q != "" {
		projectID, err := strconv.Atoi(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, web.NewErrorResponse("invalid projectId"))
			return
		}
		scoped := make([]oktedi.AttendanceRow, 0, len(rows))
		for _, r := range rows {
			if int(r.ProjectID) == projectID {
				scoped = append(scoped, r)
			}
		}
		rows = scoped
	}

	pdfBytes, err := oktedi.EvacuationRegisterPDF(rows, date, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// Audit hook (future): generation is a pure []byte, so a follow-up can insert
	// an evacuation_register_exports record (date, projectId, generatedBy,
	// generatedAt) and archive pdfBytes here before streaming, without changing
	// the renderer.
	filename := "Evacuation-Register-" + date.Format("2006-01-02") + ".pdf"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
