package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ProjectsList отображает список проектов
func ProjectsList(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var projects []models.Project
	if user.IsAdmin() {
		database.DB.Preload("CreatedBy").Order("created_at desc").Find(&projects)
	} else {
		database.DB.Preload("CreatedBy").
			Joins("JOIN project_assignments ON project_assignments.project_id = projects.id AND project_assignments.user_id = ?", user.ID).
			Order("projects.created_at desc").Find(&projects)
	}

	c.HTML(http.StatusOK, "projects.html", gin.H{
		"title":    "Проекты",
		"projects": projects,
		"user":     user,
		"active":   "projects",
	})
}

// ProjectDetail отображает детали проекта
func ProjectDetail(c *gin.Context) {
	id := c.Param("id")
	var project models.Project
	if err := database.DB.Preload("CreatedBy").Preload("Documents.UploadedBy").Preload("Estimates.CreatedBy").First(&project, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Проект не найден",
		})
		return
	}

	user := middleware.GetCurrentUser(c)

	if !userCanAccessProject(user.ID, project.ID, user.Role) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Доступ запрещён",
			"message": "У вас нет доступа к этому проекту",
		})
		return
	}

	// Назначенные пользователи
	var assignments []models.ProjectAssignment
	database.DB.Preload("User").Where("project_id = ?", project.ID).Find(&assignments)

	c.HTML(http.StatusOK, "project_detail.html", gin.H{
		"title":       project.Name,
		"project":     project,
		"user":        user,
		"active":      "projects",
		"assignments": assignments,
	})
}

// ProjectCreatePage отображает форму создания проекта
func ProjectCreatePage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "project_form.html", gin.H{
		"title":    "Новый проект",
		"statuses": models.ProjectStatuses(),
		"user":     user,
		"active":   "projects",
	})
}

// ProjectCreate обрабатывает создание проекта
func ProjectCreate(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	name := c.PostForm("name")
	description := c.PostForm("description")
	status := c.PostForm("status")
	address := c.PostForm("address")
	budgetStr := c.PostForm("budget")
	startDateStr := c.PostForm("start_date")
	endDateStr := c.PostForm("end_date")

	budget, _ := strconv.ParseFloat(budgetStr, 64)

	project := models.Project{
		Name:        name,
		Description: description,
		Status:      status,
		Address:     address,
		Budget:      budget,
		CreatedByID: user.ID,
	}

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			project.StartDate = &t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			project.EndDate = &t
		}
	}

	if err := database.DB.Create(&project).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Не удалось создать проект",
		})
		return
	}

	// Автоматически назначаем создателя на проект
	database.DB.Create(&models.ProjectAssignment{
		ProjectID:  project.ID,
		UserID:     user.ID,
		AssignedBy: user.ID,
	})

	LogAction(c, user.ID, "create_project", "project", project.ID, "Создан проект: "+name)

	c.Redirect(http.StatusFound, "/projects/"+strconv.Itoa(int(project.ID)))
}

// ProjectEditPage отображает форму редактирования проекта
func ProjectEditPage(c *gin.Context) {
	id := c.Param("id")
	var project models.Project
	if err := database.DB.First(&project, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Проект не найден",
		})
		return
	}

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "project_form.html", gin.H{
		"title":    "Редактирование: " + project.Name,
		"project":  project,
		"statuses": models.ProjectStatuses(),
		"user":     user,
		"active":   "projects",
		"editing":  true,
	})
}

// ProjectUpdate обрабатывает обновление проекта
func ProjectUpdate(c *gin.Context) {
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

	project.Name = c.PostForm("name")
	project.Description = c.PostForm("description")
	project.Status = c.PostForm("status")
	project.Address = c.PostForm("address")
	budget, _ := strconv.ParseFloat(c.PostForm("budget"), 64)
	project.Budget = budget

	startDateStr := c.PostForm("start_date")
	endDateStr := c.PostForm("end_date")

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			project.StartDate = &t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			project.EndDate = &t
		}
	}

	database.DB.Save(&project)

	LogAction(c, user.ID, "update_project", "project", project.ID, "Обновлён проект: "+project.Name)

	c.Redirect(http.StatusFound, "/projects/"+id)
}

// ProjectDelete удаляет проект (требует подтверждения жестом)
func ProjectDelete(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	id := c.Param("id")

	actionToken := c.PostForm("action_token")
	if !middleware.ValidateActionToken(actionToken, user.ID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"title":   "Отказано",
			"message": "Требуется подтверждение жестом для удаления проекта",
		})
		return
	}

	var project models.Project
	database.DB.First(&project, id)

	LogAction(c, user.ID, "delete_project", "project", project.ID,
		fmt.Sprintf("Удалён проект: %s", project.Name))

	database.DB.Delete(&models.Project{}, id)
	c.Redirect(http.StatusFound, "/dashboard#projects")
}
