package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"fmt"
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

	user := middleware.GetCurrentUser(c)
	targetUser.Status = "approved"
	database.DB.Save(&targetUser)
	LogAction(c, user.ID, "approve_user", "user", targetUser.ID, "Одобрен: "+targetUser.Email)

	c.Redirect(http.StatusFound, "/dashboard#users")
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

	user := middleware.GetCurrentUser(c)
	targetUser.Status = "rejected"
	database.DB.Save(&targetUser)
	LogAction(c, user.ID, "reject_user", "user", targetUser.ID, "Отклонён: "+targetUser.Email)

	c.Redirect(http.StatusFound, "/dashboard#users")
}

// AdminUpdateRole обновляет роль пользователя (требует подтверждения жестом)
func AdminUpdateRole(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	id := c.Param("id")
	role := c.PostForm("role")

	actionToken := c.PostForm("action_token")
	if !middleware.ValidateActionToken(actionToken, user.ID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Отказано",
			"message": "Требуется подтверждение жестом для изменения роли",
		})
		return
	}

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

	oldRole := targetUser.Role
	targetUser.Role = role
	database.DB.Save(&targetUser)
	LogAction(c, user.ID, "change_role", "user", targetUser.ID,
		fmt.Sprintf("Роль %s -> %s для %s", oldRole, role, targetUser.Email))

	c.Redirect(http.StatusFound, "/dashboard#users")
}

// AdminDeleteUser удаляет пользователя (требует подтверждения жестом)
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

	actionToken := c.PostForm("action_token")
	if !middleware.ValidateActionToken(actionToken, currentUser.ID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Отказано",
			"message": "Требуется подтверждение жестом для удаления пользователя",
		})
		return
	}

	var targetUser models.User
	database.DB.First(&targetUser, id)
	LogAction(c, currentUser.ID, "delete_user", "user", targetUser.ID, "Удалён: "+targetUser.Email)

	database.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusFound, "/dashboard#users")
}

// Dashboard отображает главную страницу
func Dashboard(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	// Проекты (для админа — все, для остальных — только назначенные)
	var projects []models.Project
	if user.IsAdmin() {
		database.DB.Preload("CreatedBy").Order("created_at desc").Find(&projects)
	} else {
		database.DB.Preload("CreatedBy").
			Joins("JOIN project_assignments ON project_assignments.project_id = projects.id AND project_assignments.user_id = ?", user.ID).
			Order("projects.created_at desc").Find(&projects)
	}

	// Сметы
	var estimates []models.Estimate
	database.DB.Preload("CreatedBy").Preload("Project").Order("created_at desc").Find(&estimates)

	// Все документы (проектные + корпоративные)
	var documents []models.Document
	database.DB.Preload("UploadedBy").Preload("Project").Order("created_at desc").Find(&documents)

	// Пользователи и журнал (только для админа)
	var pendingUsers []models.User
	var approvedUsers []models.User
	var recentLogs []models.AuditLog
	if user.IsAdmin() {
		database.DB.Where("status = ?", "pending").Order("created_at desc").Find(&pendingUsers)
		database.DB.Where("status = ?", "approved").Order("created_at desc").Find(&approvedUsers)
		database.DB.Preload("User").Order("created_at desc").Limit(10).Find(&recentLogs)
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
		"recentLogs":     recentLogs,
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
