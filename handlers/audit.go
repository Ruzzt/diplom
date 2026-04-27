package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// LogAction записывает действие в журнал аудита
func LogAction(c *gin.Context, userID uint, action, targetType string, targetID uint, details string) {
	log := models.AuditLog{
		UserID:     userID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		IP:         c.ClientIP(),
	}
	database.DB.Create(&log)
}

// AuditLogPage отображает полный журнал действий
func AuditLogPage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 50

	var total int64
	database.DB.Model(&models.AuditLog{}).Count(&total)

	var logs []models.AuditLog
	database.DB.Preload("User").
		Order("created_at desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs)

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	c.HTML(http.StatusOK, "audit_log.html", gin.H{
		"title":      "Журнал действий",
		"user":       user,
		"logs":       logs,
		"page":       page,
		"totalPages": totalPages,
		"total":      total,
	})
}
