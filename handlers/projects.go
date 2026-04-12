package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ProjectsList отображает список проектов
func ProjectsList(c *gin.Context) {
	var projects []models.Project
	database.DB.Preload("CreatedBy").Order("created_at desc").Find(&projects)

	user := middleware.GetCurrentUser(c)
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
	c.HTML(http.StatusOK, "project_detail.html", gin.H{
		"title":   project.Name,
		"project": project,
		"user":    user,
		"active":  "projects",
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
	c.Redirect(http.StatusFound, "/projects/"+id)
}

// ProjectDelete удаляет проект
func ProjectDelete(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.Project{}, id)
	c.Redirect(http.StatusFound, "/projects")
}
