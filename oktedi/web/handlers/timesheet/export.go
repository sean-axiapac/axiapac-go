package timesheet

import (
	"fmt"
	"net/http"

	web "axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

func (ep *Endpoint) Export(c *gin.Context) {
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

	// Use a sufficiently large limit to get all timesheets
	timesheets, _, err := SearchTimesheets(db, searchParams, -1, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	sheetName := "Timesheets"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{
		"Date",
		"Employee Code",
		"First Name",
		"Surname",
		"Assigned Project & WBS",
		"Actual Project & WBS",
		"Start Time",
		"Finish Time",
		"Break (mins)",
		"Core Hours",
		"Total Hours",
		"Review Status",
		"Approved",
		"Notes",
	}

	maxWidths := make([]int, len(headers))

	for i, header := range headers {
		col := string(rune('A' + i))
		f.SetCellValue(sheetName, fmt.Sprintf("%s1", col), header)
		maxWidths[i] = len(header)
	}

	// Make headers bold
	style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err == nil {
		f.SetRowStyle(sheetName, 1, 1, style)
	}

	for i, ts := range timesheets {
		row := i + 2

		assignedParts := []string{}
		if ts.Employee.Job.JobNo != "" {
			assignedParts = append(assignedParts, ts.Employee.Job.JobNo)
		}
		if ts.Employee.CostCentre.Code != "" {
			assignedParts = append(assignedParts, ts.Employee.CostCentre.Code)
		}
		assignedStr := ""
		if len(assignedParts) > 0 {
			assignedStr = fmt.Sprintf("%s", assignedParts[0])
			if len(assignedParts) > 1 {
				assignedStr += "/" + assignedParts[1]
			}
		}

		actualParts := []string{}
		if ts.Job.JobNo != "" {
			actualParts = append(actualParts, ts.Job.JobNo)
		}
		if ts.CostCentre.Code != "" {
			actualParts = append(actualParts, ts.CostCentre.Code)
		}
		actualStr := ""
		if len(actualParts) > 0 {
			actualStr = fmt.Sprintf("%s", actualParts[0])
			if len(actualParts) > 1 {
				actualStr += "/" + actualParts[1]
			}
		}

		startTimeStr := ""
		if !ts.StartTime.IsZero() {
			startTimeStr = ts.StartTime.Format("15:04")
		}

		finishTimeStr := ""
		if !ts.FinishTime.IsZero() {
			finishTimeStr = ts.FinishTime.Format("15:04")
		}

		breakMins := int32(0)
		if ts.Break != nil {
			breakMins = *ts.Break
		}

		approvedStr := "No"
		if ts.Approved {
			approvedStr = "Yes"
		}

		rowValues := []interface{}{
			ts.Date.Format("2006-01-02"),
			ts.Employee.Code,
			ts.Employee.FirstName,
			ts.Employee.Surname,
			assignedStr,
			actualStr,
			startTimeStr,
			finishTimeStr,
			breakMins,
			ts.Hours,
			ts.TotalHours,
			ts.ReviewStatus,
			approvedStr,
			ts.Notes,
		}

		for cIdx, val := range rowValues {
			col := string(rune('A' + cIdx))
			f.SetCellValue(sheetName, fmt.Sprintf("%s%d", col, row), val)

			strVal := fmt.Sprintf("%v", val)
			if len(strVal) > maxWidths[cIdx] {
				maxWidths[cIdx] = len(strVal)
			}
		}
	}

	for i, width := range maxWidths {
		col := string(rune('A' + i))
		f.SetColWidth(sheetName, col, col, float64(width)+2.0)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=timesheets.xlsx")
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(fmt.Sprintf("failed to write excel file: %v", err)))
		return
	}
}
