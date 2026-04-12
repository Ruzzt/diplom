package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// EstimatesList отображает список смет для проекта
func EstimatesList(c *gin.Context) {
	var estimates []models.Estimate
	database.DB.Preload("CreatedBy").Preload("Project").Order("created_at desc").Find(&estimates)

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "estimates.html", gin.H{
		"title":     "Сметы",
		"estimates": estimates,
		"user":      user,
		"active":    "estimates",
	})
}

// EstimateDetail отображает детали сметы
func EstimateDetail(c *gin.Context) {
	id := c.Param("id")
	var estimate models.Estimate
	if err := database.DB.Preload("Items").Preload("Project").Preload("CreatedBy").First(&estimate, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Смета не найдена",
		})
		return
	}

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "estimate_detail.html", gin.H{
		"title":    estimate.Name,
		"estimate": estimate,
		"user":     user,
		"active":   "estimates",
	})
}

// EstimateCreatePage отображает форму создания сметы
func EstimateCreatePage(c *gin.Context) {
	var projects []models.Project
	database.DB.Find(&projects)

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "estimate_form.html", gin.H{
		"title":    "Новая смета",
		"projects": projects,
		"units":    models.Units(),
		"user":     user,
		"active":   "estimates",
	})
}

type EstimateItemInput struct {
	Name      string  `json:"name"`
	Unit      string  `json:"unit"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type EstimateCreateRequest struct {
	ProjectID uint                `json:"project_id"`
	Name      string              `json:"name"`
	Items     []EstimateItemInput `json:"items"`
}

// EstimateCreate обрабатывает создание сметы (JSON API)
func EstimateCreate(c *gin.Context) {
	var req EstimateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	user := middleware.GetCurrentUser(c)

	var totalAmount float64
	var items []models.EstimateItem
	for _, item := range req.Items {
		total := item.Quantity * item.UnitPrice
		totalAmount += total
		items = append(items, models.EstimateItem{
			Name:       item.Name,
			Unit:       item.Unit,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: total,
		})
	}

	estimate := models.Estimate{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		TotalAmount: totalAmount,
		Items:       items,
		CreatedByID: user.ID,
	}

	if err := database.DB.Create(&estimate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания сметы"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Смета создана",
		"id":      estimate.ID,
	})
}

// EstimateEditPage отображает форму редактирования сметы
func EstimateEditPage(c *gin.Context) {
	id := c.Param("id")
	var estimate models.Estimate
	if err := database.DB.Preload("Items").First(&estimate, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Смета не найдена",
		})
		return
	}

	var projects []models.Project
	database.DB.Find(&projects)

	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "estimate_form.html", gin.H{
		"title":    "Редактирование: " + estimate.Name,
		"estimate": estimate,
		"projects": projects,
		"units":    models.Units(),
		"user":     user,
		"active":   "estimates",
		"editing":  true,
	})
}

// EstimateUpdate обрабатывает обновление сметы (JSON API)
func EstimateUpdate(c *gin.Context) {
	id := c.Param("id")
	var estimate models.Estimate
	if err := database.DB.First(&estimate, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Смета не найдена"})
		return
	}

	var req EstimateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	// Удаляем старые позиции
	database.DB.Where("estimate_id = ?", estimate.ID).Delete(&models.EstimateItem{})

	var totalAmount float64
	var items []models.EstimateItem
	for _, item := range req.Items {
		total := item.Quantity * item.UnitPrice
		totalAmount += total
		items = append(items, models.EstimateItem{
			EstimateID: estimate.ID,
			Name:       item.Name,
			Unit:       item.Unit,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: total,
		})
	}

	estimate.Name = req.Name
	estimate.ProjectID = req.ProjectID
	estimate.TotalAmount = totalAmount
	database.DB.Save(&estimate)

	for i := range items {
		database.DB.Create(&items[i])
	}

	idNum, _ := strconv.Atoi(id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Смета обновлена",
		"id":      idNum,
	})
}

// EstimateDelete удаляет смету
func EstimateDelete(c *gin.Context) {
	id := c.Param("id")
	database.DB.Where("estimate_id = ?", id).Delete(&models.EstimateItem{})
	database.DB.Delete(&models.Estimate{}, id)
	c.Redirect(http.StatusFound, "/estimates")
}
