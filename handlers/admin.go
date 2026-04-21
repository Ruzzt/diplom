package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminUsers отображает список пользователей
func AdminUsers(c *gin.Context) {
	var pendingUsers []models.User
	var approvedUsers []models.User

	database.DB.Where("status = ?", "pending").Order("created_at desc").Find(&pendingUsers)
	database.DB.Where("status = ?", "approved").Order("created_at desc").Find(&approvedUsers)

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"title":         "Управление пользователями",
		"pendingUsers":  pendingUsers,
		"approvedUsers": approvedUsers,
		"user":          user,
		"active":        "admin",
	})
}

// AdminApproveUser одобряет заявку пользователя
func AdminApproveUser(c *gin.Context) {
	id := c.Param("id")

	var targetUser models.User
	if err := database.DB.First(&targetUser, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Пользователь не найден",
		})
		return
	}

	targetUser.Status = "approved"
	database.DB.Save(&targetUser)

	c.Redirect(http.StatusFound, "/admin/users")
}

// AdminRejectUser отклоняет заявку пользователя
func AdminRejectUser(c *gin.Context) {
	id := c.Param("id")

	var targetUser models.User
	if err := database.DB.First(&targetUser, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Пользователь не найден",
		})
		return
	}

	targetUser.Status = "rejected"
	database.DB.Save(&targetUser)

	c.Redirect(http.StatusFound, "/admin/users")
}

// AdminUpdateRole обновляет роль пользователя
func AdminUpdateRole(c *gin.Context) {
	id := c.Param("id")
	role := c.PostForm("role")

	validRoles := map[string]bool{
		"admin": true, "director": true, "accountant": true,
		"manager": true, "engineer": true, "viewer": true,
	}
	if !validRoles[role] {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Недопустимая роль",
		})
		return
	}

	var targetUser models.User
	if err := database.DB.First(&targetUser, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Пользователь не найден",
		})
		return
	}

	targetUser.Role = role
	database.DB.Save(&targetUser)

	c.Redirect(http.StatusFound, "/admin/users")
}

// AdminDeleteUser удаляет пользователя
func AdminDeleteUser(c *gin.Context) {
	id := c.Param("id")

	currentUser := middleware.GetCurrentUser(c)
	if currentUser != nil && currentUser.ID == parseUint(id) {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Нельзя удалить самого себя",
		})
		return
	}

	database.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusFound, "/admin/users")
}

// Dashboard отображает главную страницу
func Dashboard(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Проекты
	var projects []models.Project
	database.DB.Preload("CreatedBy").Order("created_at desc").Find(&projects)

	// Сметы
	var estimates []models.Estimate
	database.DB.Preload("CreatedBy").Preload("Project").Order("created_at desc").Find(&estimates)

	// Все документы (проектные + корпоративные)
	var documents []models.Document
	database.DB.Preload("UploadedBy").Preload("Project").Order("created_at desc").Find(&documents)

	// Пользователи (только для админа)
	var pendingUsers []models.User
	var approvedUsers []models.User
	if user.IsAdmin() {
		database.DB.Where("status = ?", "pending").Order("created_at desc").Find(&pendingUsers)
		database.DB.Where("status = ?", "approved").Order("created_at desc").Find(&approvedUsers)
	}

	// Аналитика по статусам проектов
	statusCounts := map[string]int{
		"planning": 0, "active": 0, "completed": 0, "suspended": 0,
	}
	for _, p := range projects {
		statusCounts[p.Status]++
	}

	// Общий бюджет
	var totalBudget float64
	for _, p := range projects {
		totalBudget += p.Budget
	}

	// Общая сумма смет
	var totalEstimates float64
	for _, e := range estimates {
		totalEstimates += e.TotalAmount
	}

	// Средний бюджет
	var avgBudget float64
	if len(projects) > 0 {
		avgBudget = totalBudget / float64(len(projects))
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":         "Панель управления",
		"user":          user,
		"active":        "dashboard",
		"projects":      projects,
		"estimates":     estimates,
		"documents":     documents,
		"pendingUsers":  pendingUsers,
		"approvedUsers": approvedUsers,
		"projectCount":  int64(len(projects)),
		"estimateCount": int64(len(estimates)),
		"documentCount": int64(len(documents)),
		"userCount":     int64(len(approvedUsers) + len(pendingUsers)),
		// Аналитика
		"planningCount":  statusCounts["planning"],
		"activeCount":    statusCounts["active"],
		"completedCount": statusCounts["completed"],
		"suspendedCount": statusCounts["suspended"],
		"totalBudget":    totalBudget,
		"totalEstimates": totalEstimates,
		"avgBudget":      avgBudget,
	})
}

func parseUint(s string) uint {
	var result uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + uint(c-'0')
		}
	}
	return result
}
