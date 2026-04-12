package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole проверяет, что пользователь имеет одну из указанных ролей
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
			c.Abort()
			return
		}

		role := userRole.(string)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}

		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Доступ запрещён",
			"message": "У вас нет прав для выполнения этого действия",
		})
		c.Abort()
	}
}

// RequireAdmin проверяет, что пользователь — администратор
func RequireAdmin() gin.HandlerFunc {
	return RequireRole("admin")
}

// RequireEditor проверяет, что пользователь может редактировать (admin, director или manager)
func RequireEditor() gin.HandlerFunc {
	return RequireRole("admin", "director", "manager")
}
