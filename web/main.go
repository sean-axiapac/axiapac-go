package main

import (
	"log"

	"axiapac.com/axiapac/core"
	"axiapac.com/axiapac/web/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	dm, err := core.New("root:development@tcp(localhost:3306)/development?parseTime=true", 10)
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

	protected := r.Group("/api")
	// protected.Use(middlewares.Authentication())
	{
		protected.GET("/hello", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			c.JSON(200, gin.H{
				"message": "Hello device!",
				"claims":  claims,
			})
		})

	}

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
