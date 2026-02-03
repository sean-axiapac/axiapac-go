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
	r.GET("/timesheets/:id", endpoint.Get)
	r.PUT("/timesheets/:id", endpoint.Update)
	// r.GET("/owner-disbursments/:id", endpoint.Find)
	// r.GET("/owner-disbursments/:id/statements", endpoint.ListStatements)

	// convert records to oktedi timesheets
	r.POST("/timesheets/prepare", endpoint.Prepare)
}

type OktediTimesheetUpdateDTO struct {
	Hours        *float64           `json:"hours,omitempty"`
	StartTime    *web.LocalDateTime `json:"startTime,omitempty"`
	FinishTime   *web.LocalDateTime `json:"finishTime,omitempty"`
	ReviewStatus *string            `json:"reviewStatus,omitempty"`
	Approved     *bool              `json:"approved,omitempty"`
	Break        *int32             `json:"break,omitempty"`
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
	var oldTs model.OktediTimesheet
	if err := db.First(&oldTs, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// Convert DTO to map for GORM to handle custom LocalDateTime correctly
	updates := make(map[string]interface{})
	if updateDTO.Hours != nil {
		updates["hours"] = *updateDTO.Hours
	}
	if updateDTO.StartTime != nil {
		updates["start_time"] = updateDTO.StartTime.Time
	}
	if updateDTO.FinishTime != nil {
		updates["finish_time"] = updateDTO.FinishTime.Time
	}
	if updateDTO.ReviewStatus != nil {
		updates["review_status"] = *updateDTO.ReviewStatus
	}
	if updateDTO.Approved != nil {
		updates["approved"] = *updateDTO.Approved
	}
	if updateDTO.Break != nil {
		updates["break"] = *updateDTO.Break
	}

	// Update the timesheet in the database
	if err := db.Model(&model.OktediTimesheet{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// If approved, sync to Axiapac
	var ts model.OktediTimesheet
	if err := db.First(&ts, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, web.NewErrorResponse(err.Error()))
		return
	}

	// Trigger sync only if it's currently approved AND it was previously not approved
	if ts.Approved && !oldTs.Approved {
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
