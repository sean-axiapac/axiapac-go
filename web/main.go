package main

import (
	"axiapac.com/axiapac/web/handlers"
	"axiapac.com/axiapac/web/middlewares"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/upload/multiple", handlers.UploadMultipleHandler)

	protected := r.Group("/api")
	protected.Use(middlewares.Authentication())
	{
		protected.GET("/hello", func(c *gin.Context) {
			claims, _ := c.Get("claims")
			c.JSON(200, gin.H{
				"message": "Hello device!",
				"claims":  claims,
			})
		})
	}

	r.Run(":8090")
}
