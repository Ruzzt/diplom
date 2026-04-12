package main

import (
	"encoding/json"
	"face-auth-system/config"
	"face-auth-system/database"
	"face-auth-system/handlers"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

func main() {
	// Загрузка конфигурации
	cfg := config.Load()

	// Подключение к БД
	database.Connect(cfg)
	database.Migrate()

	// Настройка JWT
	middleware.SetJWTSecret(cfg.JWTSecret)

	// Настройка Gin
	r := gin.Default()

	// Шаблонные функции и загрузка шаблонов
	funcMap := template.FuncMap{
		"formatDate":      formatDate,
		"formatDateInput": formatDateInput,
		"formatMoney":     formatMoney,
		"formatSize":      formatSize,
		"statusColor":     statusColor,
		"statusLabel":     models.ProjectStatusLabel,
		"docTypeLabel":    models.DocTypeLabel,
		"truncate":        truncate,
		"inc":             func(i int) int { return i + 1 },
		"marshal":         marshalJSON,
		"eq": func(a, b interface{}) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		},
		"roleLabel":       models.RoleLabel,
		"userStatusLabel": models.StatusLabelUser,
		"projectName": func(doc models.Document) string {
			if doc.ProjectID == nil || *doc.ProjectID == 0 {
				return "Корпоративный"
			}
			return doc.Project.Name
		},
	}
	r.HTMLRender = loadTemplates(funcMap)

	// Статические файлы
	r.Static("/static", "./static")

	// --- Публичные маршруты ---
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "welcome.html", gin.H{
			"title": "Добро пожаловать",
		})
	})
	r.GET("/login", handlers.LoginPage)
	r.GET("/register", handlers.RegisterPage)
	r.GET("/logout", handlers.Logout)

	// API аутентификации (без JWT)
	r.POST("/api/register", handlers.Register)
	r.POST("/api/login", handlers.Login)
	r.GET("/api/gesture-challenge", handlers.GestureChallenge)

	// --- Защищённые маршруты ---
	auth := r.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		// Дашборд
		auth.GET("/dashboard", handlers.Dashboard)

		// Проекты — просмотр (все роли)
		auth.GET("/projects", handlers.ProjectsList)
		auth.GET("/projects/:id", handlers.ProjectDetail)

		// Проекты — редактирование (admin, manager)
		editor := auth.Group("/")
		editor.Use(middleware.RequireEditor())
		{
			editor.GET("/projects/new", handlers.ProjectCreatePage)
			editor.POST("/projects", handlers.ProjectCreate)
			editor.GET("/projects/:id/edit", handlers.ProjectEditPage)
			editor.POST("/projects/:id/edit", handlers.ProjectUpdate)
			editor.POST("/projects/:id/delete", handlers.ProjectDelete)

			// Документы проектов
			editor.POST("/projects/:id/documents", handlers.DocumentUpload)
			editor.POST("/projects/:id/documents/:docId/delete", handlers.DocumentDelete)

			// Корпоративные документы
			editor.POST("/documents/upload", handlers.CorporateDocumentUpload)
			editor.POST("/documents/:docId/delete", handlers.CorporateDocumentDelete)
		}

		// Документы — скачивание (все роли)
		auth.GET("/projects/:id/documents/:docId/download", handlers.DocumentDownload)
		auth.GET("/documents/:docId/download", handlers.DocumentDownloadByID)

		// Сметы — просмотр (все роли)
		auth.GET("/estimates", handlers.EstimatesList)
		auth.GET("/estimates/:id", handlers.EstimateDetail)

		// Сметы — редактирование (admin, manager)
		editorEst := auth.Group("/")
		editorEst.Use(middleware.RequireEditor())
		{
			editorEst.GET("/estimates/new", handlers.EstimateCreatePage)
			editorEst.GET("/estimates/:id/edit", handlers.EstimateEditPage)
			editorEst.POST("/estimates/:id/delete", handlers.EstimateDelete)

			// Сметы API
			editorEst.POST("/api/estimates", handlers.EstimateCreate)
			editorEst.PUT("/api/estimates/:id", handlers.EstimateUpdate)
		}

		// Админ-панель (только admin)
		admin := auth.Group("/admin")
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/users", handlers.AdminUsers)
			admin.POST("/users/:id/role", handlers.AdminUpdateRole)
			admin.POST("/users/:id/approve", handlers.AdminApproveUser)
			admin.POST("/users/:id/reject", handlers.AdminRejectUser)
			admin.POST("/users/:id/delete", handlers.AdminDeleteUser)
		}
	}

	// Запуск сервера
	log.Printf("Сервер запущен на http://localhost:%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatal("Ошибка запуска сервера: ", err)
	}
}

// Рендерер шаблонов — каждая страница парсится отдельно с layout,
// чтобы блоки {{ define "content" }} не конфликтовали между файлами.

type templateRenderer struct {
	templates map[string]*template.Template
}

func loadTemplates(funcMap template.FuncMap) *templateRenderer {
	r := &templateRenderer{templates: make(map[string]*template.Template)}

	pages, err := filepath.Glob("templates/*.html")
	if err != nil {
		log.Fatal("Ошибка поиска шаблонов: ", err)
	}

	for _, page := range pages {
		name := filepath.Base(page)
		if name == "layout.html" {
			continue
		}
		r.templates[name] = template.Must(
			template.New(name).Funcs(funcMap).ParseFiles("templates/layout.html", page),
		)
	}

	return r
}

func (t *templateRenderer) Instance(name string, data any) render.Render {
	tmpl, ok := t.templates[name]
	if !ok {
		log.Fatalf("Шаблон %s не найден", name)
	}
	return render.HTML{
		Template: tmpl,
		Name:     "layout",
		Data:     data,
	}
}

// Шаблонные функции

func formatDate(t interface{}) string {
	switch v := t.(type) {
	case time.Time:
		return v.Format("02.01.2006")
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("02.01.2006")
	default:
		return ""
	}
}

func formatDateInput(t interface{}) string {
	switch v := t.(type) {
	case time.Time:
		return v.Format("2006-01-02")
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("2006-01-02")
	default:
		return ""
	}
}

func formatMoney(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d Б", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f КБ", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1f МБ", float64(size)/(1024*1024))
	}
}

func statusColor(status string) string {
	colors := map[string]string{
		"planning":  "planning",
		"active":    "active",
		"completed": "completed",
		"suspended": "suspended",
	}
	if color, ok := colors[status]; ok {
		return color
	}
	return "secondary"
}

func truncate(s string, length int) string {
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	return string(runes[:length]) + "..."
}

func marshalJSON(v interface{}) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(b)
}
