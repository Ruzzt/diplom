package handlers

import (
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// DocumentUpload обрабатывает загрузку документа
func DocumentUpload(c *gin.Context) {
	projectID := c.Param("id")

	var project models.Project
	if err := database.DB.First(&project, projectID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Проект не найден",
		})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Файл не выбран",
		})
		return
	}

	user := middleware.GetCurrentUser(c)
	docType := c.PostForm("doc_type")
	docName := c.PostForm("name")
	if docName == "" {
		docName = file.Filename
	}

	// Создаём директорию для проекта
	uploadDir := filepath.Join("uploads", projectID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Не удалось создать директорию",
		})
		return
	}

	// Уникальное имя файла
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	filePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Не удалось сохранить файл",
		})
		return
	}

	pid, _ := strconv.ParseUint(projectID, 10, 32)
	pidUint := uint(pid)
	doc := models.Document{
		ProjectID:    &pidUint,
		Name:         docName,
		FilePath:     filePath,
		DocType:      docType,
		FileSize:     file.Size,
		UploadedByID: user.ID,
	}

	database.DB.Create(&doc)
	c.Redirect(http.StatusFound, "/projects/"+projectID)
}

// DocumentDownload обрабатывает скачивание документа
func DocumentDownload(c *gin.Context) {
	id := c.Param("docId")
	var doc models.Document
	if err := database.DB.First(&doc, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Документ не найден",
		})
		return
	}

	c.FileAttachment(doc.FilePath, doc.Name)
}

// DocumentDelete удаляет документ
func DocumentDelete(c *gin.Context) {
	projectID := c.Param("id")
	docID := c.Param("docId")

	var doc models.Document
	if err := database.DB.First(&doc, docID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Документ не найден",
		})
		return
	}

	// Удаляем файл с диска
	os.Remove(doc.FilePath)
	database.DB.Delete(&doc)

	c.Redirect(http.StatusFound, "/projects/"+projectID)
}

// CorporateDocumentUpload загружает корпоративный документ (без привязки к проекту)
func CorporateDocumentUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Файл не выбран",
		})
		return
	}

	user := middleware.GetCurrentUser(c)
	docType := c.PostForm("doc_type")
	docName := c.PostForm("name")
	if docName == "" {
		docName = file.Filename
	}

	uploadDir := filepath.Join("uploads", "corporate")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Не удалось создать директорию",
		})
		return
	}

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
	filePath := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"title":   "Ошибка",
			"message": "Не удалось сохранить файл",
		})
		return
	}

	doc := models.Document{
		ProjectID:    nil,
		Name:         docName,
		FilePath:     filePath,
		DocType:      docType,
		FileSize:     file.Size,
		UploadedByID: user.ID,
	}

	database.DB.Create(&doc)
	c.Redirect(http.StatusFound, "/dashboard#documents")
}

// CorporateDocumentDelete удаляет корпоративный документ
func CorporateDocumentDelete(c *gin.Context) {
	docID := c.Param("docId")
	var doc models.Document
	if err := database.DB.First(&doc, docID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Документ не найден",
		})
		return
	}
	os.Remove(doc.FilePath)
	database.DB.Delete(&doc)
	c.Redirect(http.StatusFound, "/dashboard#documents")
}

// DocumentDownloadByID скачивает документ по ID (без привязки к проекту)
func DocumentDownloadByID(c *gin.Context) {
	id := c.Param("docId")
	var doc models.Document
	if err := database.DB.First(&doc, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title":   "Не найдено",
			"message": "Документ не найден",
		})
		return
	}
	c.FileAttachment(doc.FilePath, doc.Name)
}
