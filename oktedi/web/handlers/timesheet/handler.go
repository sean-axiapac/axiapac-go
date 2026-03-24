package timesheet

import (
	"fmt"
	"net/http"
	"strconv"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/core/models"
	oktedi "axiapac.com/axiapac/oktedi/core"
	"axiapac.com/axiapac/oktedi/model"
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
	r.POST("/timesheets/search", endpoint.Search)
	r.POST("/timesheets/export", endpoint.Export)
	r.GET("/timesheets/:id", endpoint.Get)
	r.PUT("/timesheets/:id", endpoint.Update)
	r.POST("/timesheets/prepare", endpoint.Prepare)
	r.POST("/timesheets/sign-off", endpoint.SignOff)
}

type OktediTimesheetUpdateDTO struct {
	Hours        *float64           `json:"hours,omitempty"`
	StartTime    *web.LocalDateTime `json:"startTime,omitempty"`
	FinishTime   *web.LocalDateTime `json:"finishTime,omitempty"`
	ReviewStatus *string            `json:"reviewStatus,omitempty"`
	Approved     *bool              `json:"approved,omitempty"`
	Break        *int32             `json:"break,omitempty"`
	ProjectID    *int32             `json:"projectId,omitempty"`
	CostCentreID *int32             `json:"costCentreId,omitempty"`
	Notes        *string            `json:"notes,omitempty"`
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

	// Get the old state of the timesheet
	var ts model.OktediTimesheet
	if err := db.First(&ts, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}
	wasApproved := ts.Approved

	// Update the timesheet object from DTO
	if updateDTO.Hours != nil {
		ts.Hours = *updateDTO.Hours
	}
	if updateDTO.StartTime != nil {
		ts.StartTime = updateDTO.StartTime.Time
	}
	if updateDTO.FinishTime != nil {
		ts.FinishTime = updateDTO.FinishTime.Time
	}
	if updateDTO.ReviewStatus != nil {
		ts.ReviewStatus = *updateDTO.ReviewStatus
	}
	if updateDTO.Approved != nil {
		ts.Approved = *updateDTO.Approved
	}
	if updateDTO.Break != nil {
		ts.Break = updateDTO.Break
	}
	if updateDTO.ProjectID != nil {
		ts.ProjectID = updateDTO.ProjectID
	}
	if updateDTO.CostCentreID != nil {
		ts.CostCentreID = updateDTO.CostCentreID
	}
	if updateDTO.Notes != nil {
		ts.Notes = *updateDTO.Notes
	}

	// Recalculate review status
	if ts.ReviewStatus != "accurate" {
		if err := oktedi.RefreshReviewStatus(db, &ts); err != nil {
			c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
			return
		}
	}

	// Save the timesheet back to the database
	if err := db.Save(&ts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// Trigger sync only if it's currently approved AND it was previously not approved
	if ts.Approved && !wasApproved {
		// Get logged in user id from claims
		claims, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, web.NewErrorResponse("claims not found"))
			return
		}

		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, web.NewErrorResponse("invalid claims format"))
			return
		}

		userIDVal, ok := mapClaims["nameid"]
		if !ok {
			c.JSON(http.StatusUnauthorized, web.NewErrorResponse("nameid not found in claims"))
			return
		}

		// userID can be string or float64 depending on how it was parsed
		var userID int32
		switch v := userIDVal.(type) {
		case float64:
			userID = int32(v)
		case string:
			idInt, _ := strconv.Atoi(v)
			userID = int32(idInt)
		default:
			c.JSON(http.StatusUnauthorized, web.NewErrorResponse("invalid userID type in claims"))
			return
		}

		// Fetch user to create client
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusOK, web.NewErrorResponse(fmt.Sprintf("failed to fetch user: %v", err)))
			return
		}

		hostname := common.GetHostname(c.Request.Host)
		client, err := oktedi.CreateClient(&user, hostname)
		if err != nil {
			c.JSON(http.StatusOK, web.NewErrorResponse(fmt.Sprintf("failed to create asiapac client: %v", err)))
			return
		}

		if err := oktedi.SyncOktediTimesheet(db, client, &ts); err != nil {
			c.JSON(http.StatusOK, web.NewErrorResponse(fmt.Sprintf("failed to sync timesheet: %v", err)))
			return
		}
	}

	c.JSON(http.StatusOK, web.NewSuccessResponse(gin.H{}))
}
