package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// userCanAccessProject проверяет, есть ли у пользователя доступ к проекту
func userCanAccessProject(userID uint, projectID uint, userRole string) bool {
	if userRole == "admin" {
		return true
	}
	var count int64
	database.DB.Model(&models.ProjectAssignment{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count)
	return count > 0
}

// ProjectAssignmentsPage отображает страницу назначений на проект
func ProjectAssignmentsPage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	id := c.Param("id")

	var project models.Project
	if err := database.DB.First(&project, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Проект не найден",
		})
		return
	}

	// Текущие назначения
	var assignments []models.ProjectAssignment
	database.DB.Preload("User").Where("project_id = ?", project.ID).Find(&assignments)

	// Все одобренные пользователи
	var allUsers []models.User
	database.DB.Where("status = ?", "approved").Order("name").Find(&allUsers)

	// Исключаем уже назначенных
	assignedMap := make(map[uint]bool)
	for _, a := range assignments {
		assignedMap[a.UserID] = true
	}
	var availableUsers []models.User
	for _, u := range allUsers {
		if !assignedMap[u.ID] {
			availableUsers = append(availableUsers, u)
		}
	}

	c.HTML(http.StatusOK, "project_assignments.html", gin.H{
		"title":          "Назначения — " + project.Name,
		"user":           user,
		"project":        project,
		"assignments":    assignments,
		"availableUsers": availableUsers,
	})
}

// ProjectAssignUser назначает пользователя на проект
func ProjectAssignUser(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	projectID, _ := strconv.Atoi(c.Param("id"))
	userIDStr := c.PostForm("user_id")
	targetUserID, _ := strconv.Atoi(userIDStr)

	if targetUserID == 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/projects/%d/assignments", projectID))
		return
	}

	assignment := models.ProjectAssignment{
		ProjectID:  uint(projectID),
		UserID:     uint(targetUserID),
		AssignedBy: user.ID,
	}
	database.DB.Create(&assignment)

	var targetUser models.User
	database.DB.First(&targetUser, targetUserID)
	LogAction(c, user.ID, "assign_user", "project", uint(projectID),
		fmt.Sprintf("Назначен %s на проект", targetUser.Name))

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/projects/%d/assignments", projectID))
}

// ProjectUnassignUser снимает пользователя с проекта
func ProjectUnassignUser(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	projectID, _ := strconv.Atoi(c.Param("id"))
	targetUserID, _ := strconv.Atoi(c.Param("userId"))

	database.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).
		Delete(&models.ProjectAssignment{})

	var targetUser models.User
	database.DB.First(&targetUser, targetUserID)
	LogAction(c, user.ID, "unassign_user", "project", uint(projectID),
		fmt.Sprintf("Снят %s с проекта", targetUser.Name))

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/projects/%d/assignments", projectID))
}
