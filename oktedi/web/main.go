package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"axiapac.com/axiapac/core"
	clockin "axiapac.com/axiapac/oktedi/web/handlers"
	"axiapac.com/axiapac/oktedi/web/handlers/timesheet"
	"axiapac.com/axiapac/web/common"
	"axiapac.com/axiapac/web/handlers"
	"axiapac.com/axiapac/web/middlewares"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	r := gin.Default()
	dsn := os.Getenv("DSN")
	fmt.Printf("using DSN: %s\n", dsn)
	region := os.Getenv("AWS_REGION")
	fmt.Printf("using REGION: %s\n", region)
	dm, err := core.New(dsn, 10)

	if err != nil {
		log.Fatal(err)
	}
	defer dm.Close()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/upload/multiple", handlers.UploadMultipleHandler)
	base64Secret := os.Getenv("AXIAPAC_SIGNING_SECRET")
	jwtSecret, err := base64.StdEncoding.DecodeString(base64Secret)
	if err != nil {
		log.Fatal("Failed to decode JWT secret:", err)
	}

	r.GET("/api/oktedi/manifest/dev", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":     "1.0.0-dev",
			"description": "Oktedi API Manifest for Development",
		})
	})

	protected := r.Group("/api/oktedi/v1.0")
	protected.Use(middlewares.Authentication(jwtSecret))
	{
		protected.GET("/hello", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			c.JSON(200, gin.H{
				"message": "Hello device!",
				"claims":  claims,
			})
		})
		timesheet.Register(protected, dm)

		protected.GET("/data", func(c *gin.Context) {
			// result := ""
			// fmt.Println(input.Query)
			ctx := c.Request.Context()
			var employees []EmployeeInfo
			var jobs []JobInfo
			var costCentres []CostCentreInfo

			if err := dm.Exec(ctx, "oktedi", func(db *gorm.DB) error {
				// employees
				err := db.Table("employees").
					Select(`
		employees.employeeid as id,
        employees.identificationTag as tag,
		employees.code as code,
		employees.picture as avatar,
        employees.firstname as first_name,
        employees.surname as last_name,
		employees.jobid as job_id,
		employees.costcentreid AS cost_centre_id,
        employees.reportstoid as supervisor_id
    `).
					Scan(&employees).Error
				if err != nil {
					return err
				}
				// jobs
				err = db.Table("jobs").
					Select(`
		jobs.jobid as id,
        jobs.jobno as job_no,
        jobs.description as description
    `).
					Scan(&jobs).Error
				if err != nil {
					return err
				}
				// costcentres
				err = db.Raw(`
		SELECT 
			jcc.jobid AS job_id,
			cc.costcentreid AS id,
			cc.code AS code,
			cc.description AS description
		FROM jobcostcentres jcc JOIN costcentres cc USING (costcentreid)
    `).
					Scan(&costCentres).Error
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
				return
			}

			c.JSON(http.StatusOK, common.NewSuccessResponse(Data{Employees: employees, Jobs: jobs, CostCentres: costCentres}))
		})

		protected.GET("/supervisors/:supervisorId/assignments", clockin.SearchSupervisorRecordsHandler(dm))
		protected.POST("/supervisors/:supervisorId/assignments", clockin.SaveSupervisorRecordsHandler(dm))

		protected.GET("/whoami", func(c *gin.Context) {
			// get query param "tag"
			tag := c.Query("tag")
			ctx := c.Request.Context()
			var emp *EmployeeInfo
			var crew []EmployeeInfo
			if err := dm.Exec(ctx, "oktedi", func(db *gorm.DB) error {

				// employees
				err := db.Table("employees").
					Select(`
		employees.employeeid as id,
        employees.identificationTag as tag,
		employees.code as code,
		employees.picture as avatar,
        employees.firstname as first_name,
        employees.surname as last_name,
		employees.jobid as job_id,
		employees.costcentreid AS cost_centre_id,
        employees.reportstoid as supervisor_id
    `).
					Where("employees.identificationTag = ?", tag).
					Order("employees.employeeid").Take(&emp).Error
				if err != nil {
					return err
				}

				if emp.ID == 0 {
					emp = nil
				} else {
					// fetch crew members under this supervisor
					err := db.Table("employees").
						Select(`
							employees.employeeid as id,
							employees.identificationTag as tag,
							employees.code as code,
							employees.picture as avatar,
							employees.firstname as first_name,
							employees.surname as last_name,
							employees.jobid as job_id,
							employees.costcentreid AS cost_centre_id,
							employees.reportstoid as supervisor_id
						`).
						Where("employees.reportstoid = ?", emp.ID).
						Or(`(
							JSON_CONTAINS_PATH(employees.attributes, 'one', '$.backToBack.id')
							AND JSON_EXTRACT(employees.attributes, '$.backToBack.id') = CAST(? AS JSON)
						)`, emp.ID).
						Or(`(
							JSON_CONTAINS_PATH(employees.attributes, 'one', '$.manager.id')
							AND JSON_EXTRACT(employees.attributes, '$.manager.id') = CAST(? AS JSON)
						)`, emp.ID).
						Scan(&crew).Error
					if err != nil {
						return err
					}
				}

				return nil
			}); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
				return
			}

			isAdmin := tag == "04791BE2BC1C90" || tag == "046682620E1590" || tag == "04A245E2BC1C90" || tag == "041A55B88F6180" || tag == "30372"
			// respond with employee info and isAdmin flag
			c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{
				"employee": emp,
				"isAdmin":  isAdmin,
				"crew":     crew,
			}))
		})

		protected.PUT("/employees/:id", func(c *gin.Context) {
			// get body "tag"
			id := c.Param("id")
			var body struct {
				Tag string `json:"tag"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.FormatBindingError(err)))
				return
			}
			ctx := c.Request.Context()
			if err := dm.Exec(ctx, "oktedi", func(db *gorm.DB) error {
				result := db.Exec("UPDATE employees SET identificationTag = ? WHERE employeeid = ?", body.Tag, id)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return errors.New("no employee found with the given ID")
				}
				return nil
			}); err != nil {
				c.JSON(http.StatusInternalServerError, common.NewErrorResponse(err.Error()))
				return
			}

			c.JSON(http.StatusOK, common.NewSuccessResponse(gin.H{}))
		})

		protected.POST("/pull", func(c *gin.Context) {
			bodyBytes, err := c.GetRawData()
			if err != nil {
				c.String(http.StatusBadRequest, "Error reading body")
				return
			}

			bodyString := string(bodyBytes)
			fmt.Println(bodyString)
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		protected.POST("/push", clockin.WatermelonPushHandler(dm))

	}

	r.StaticFile("/", "./public/index.html")
	r.Static("/assets", "./public/assets")
	r.Static("/oktedi/assets", "./public/assets")

	r.GET("/oktedi", func(c *gin.Context) {
		c.File("./public/index.html")
	})

	r.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Redirect(http.StatusFound, "/oktedi")
			return
		}
	})

	r.Run("0.0.0.0:8090")
}

type Data struct {
	Employees   []EmployeeInfo   `json:"employees"`
	Jobs        []JobInfo        `json:"jobs"`
	CostCentres []CostCentreInfo `json:"costCentres"`
}

type EmployeeInfo struct {
	ID           uint    `json:"id"`
	Code         string  `json:"code"`
	Tag          string  `json:"tag"`
	FirstName    string  `json:"firstName"`
	LastName     string  `json:"lastName"`
	Avatar       *string `json:"avatar"`
	JobID        *uint   `json:"jobId"`
	CostCentreID *uint   `json:"costCentreId"`
	SupervisorID *uint   `json:"supervisorId"`
}

type JobInfo struct {
	ID          uint   `json:"id"`
	JobNo       string `json:"jobNo"`
	Description string `json:"description"`
}

type CostCentreInfo struct {
	ID          uint   `json:"id"`
	JobId       int    `json:"jobId"`
	Code        string `json:"code"`
	Description string `json:"description"`
}
