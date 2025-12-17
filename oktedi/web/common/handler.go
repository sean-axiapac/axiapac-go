package common

import (
	"database/sql"
	"net"

	"axiapac.com/axiapac/core"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	Dm *core.DatabaseManager
}

func GetHostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func (h *Handler) GetDB(r *gin.Context) (*gorm.DB, *sql.Conn, error) {
	hostname := GetHostname(r.Request.Host)
	return h.Dm.GetDB(r.Request.Context(), hostname)
}
