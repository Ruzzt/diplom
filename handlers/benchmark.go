package handlers

import (
	"encoding/json"
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"face-auth-system/recognition"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BenchmarkPage отображает страницу сравнения методов
func BenchmarkPage(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "benchmark.html", gin.H{
		"title":        "Сравнение методов распознавания",
		"user":         user,
		"hasLandmarks": user.FaceLandmarks != "" && user.FaceLandmarks != "null",
	})
}

// BenchmarkUpdateLandmarks обновляет landmarks пользователя
func BenchmarkUpdateLandmarks(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		FaceDescriptor []float64              `json:"face_descriptor"`
		FaceLandmarks  []recognition.Landmark `json:"face_landmarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if len(req.FaceLandmarks) < 68 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недостаточно точек лица (нужно 68)"})
		return
	}

	landmarksJSON, _ := json.Marshal(req.FaceLandmarks)
	descriptorJSON, _ := json.Marshal(req.FaceDescriptor)

	// Обновляем и landmarks и дескриптор
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"face_landmarks":  string(landmarksJSON),
		"face_descriptor": string(descriptorJSON),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Биометрия обновлена"})
}

// BenchmarkCompare выполняет сравнение двух методов для текущего пользователя
func BenchmarkCompare(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		FaceDescriptor []float64              `json:"face_descriptor"`
		FaceLandmarks  []recognition.Landmark `json:"face_landmarks"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if len(req.FaceLandmarks) < 68 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недостаточно точек лица"})
		return
	}

	// Получаем всех пользователей с landmarks
	var users []models.User
	database.DB.Where("status = ? AND face_landmarks IS NOT NULL AND face_landmarks != '' AND face_landmarks != 'null'", "approved").Find(&users)

	type UserResult struct {
		UserID   uint                        `json:"user_id"`
		UserName string                      `json:"user_name"`
		IsSelf   bool                        `json:"is_self"`
		Result   recognition.BenchmarkResult `json:"result"`
	}

	var results []UserResult

	for _, u := range users {
		if u.FaceDescriptor == "" {
			continue
		}

		var storedDescriptor []float64
		json.Unmarshal([]byte(u.FaceDescriptor), &storedDescriptor)

		var storedLandmarks []recognition.Landmark
		if u.FaceLandmarks != "" && u.FaceLandmarks != "null" {
			json.Unmarshal([]byte(u.FaceLandmarks), &storedLandmarks)
		}

		bench := recognition.RunBenchmark(
			req.FaceDescriptor, storedDescriptor,
			req.FaceLandmarks, storedLandmarks,
		)

		results = append(results, UserResult{
			UserID:   u.ID,
			UserName: u.Name,
			IsSelf:   u.ID == user.ID,
			Result:   bench,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}
