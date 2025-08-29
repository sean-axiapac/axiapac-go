package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// UploadMultipleHandler handles multiple photo uploads
func UploadMultipleHandler(c *gin.Context) {
	// Parse multipart form (max 50 MB)
	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	form := c.Request.MultipartForm
	files := form.File["files"] // client sends multiple files as "files[]"

	uploaded := []string{}

	for _, file := range files {
		// Optional: validate file type
		ext := filepath.Ext(file.Filename)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".pdf" {
			continue
		}

		// Save file to disk
		dst := filepath.Join("uploads", file.Filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Add to response list
		uploaded = append(uploaded, dst)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("%d files uploaded", len(uploaded)),
		"files":   uploaded,
	})
}
