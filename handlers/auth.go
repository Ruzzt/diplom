package handlers

import (
	"encoding/json"
	"face-auth-system/database"
	"face-auth-system/middleware"
	"face-auth-system/models"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// LoginPage отображает страницу входа с случайным жестом
func LoginPage(c *gin.Context) {
	gestures := []string{"thumbs_up", "peace", "open_palm", "one_finger"}
	gesture := gestures[rand.Intn(len(gestures))]

	emojis := map[string]string{
		"thumbs_up":  "👍",
		"peace":      "✌️",
		"open_palm":  "✋",
		"one_finger": "☝️",
	}
	names := map[string]string{
		"thumbs_up":  "Большой палец вверх",
		"peace":      "Знак мира (два пальца)",
		"open_palm":  "Открытая ладонь",
		"one_finger": "Один палец вверх",
	}

	// Создаём подписанный токен жеста (2 минуты)
	claims := jwt.MapClaims{
		"gesture": gesture,
		"type":    "gesture_challenge",
		"exp":     time.Now().Add(2 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(middleware.JWTSecret)

	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":        "Вход в систему",
		"gesture":      gesture,
		"gestureEmoji": emojis[gesture],
		"gestureName":  names[gesture],
		"gestureToken": tokenString,
	})
}

// RegisterPage отображает страницу регистрации
func RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "Регистрация",
	})
}

type RegisterRequest struct {
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	FaceDescriptor []float64 `json:"face_descriptor"`
}

// Register обрабатывает регистрацию нового пользователя
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if req.Name == "" || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя и email обязательны"})
		return
	}

	if len(req.FaceDescriptor) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо отсканировать лицо"})
		return
	}

	// Проверяем, нет ли пользователя с таким email
	var existing models.User
	if err := database.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Пользователь с таким email уже существует"})
		return
	}

	// Сериализуем дескриптор лица
	descriptorJSON, err := json.Marshal(req.FaceDescriptor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки данных лица"})
		return
	}

	// Валидация роли
	validRoles := map[string]bool{
		"director": true, "accountant": true, "manager": true,
		"engineer": true, "viewer": true,
	}
	role := req.Role
	if !validRoles[role] {
		role = "viewer"
	}
	// Первый пользователь — администратор с автоодобрением
	var count int64
	database.DB.Model(&models.User{}).Count(&count)
	status := "pending"
	if count == 0 {
		role = "admin"
		status = "approved"
	}

	user := models.User{
		Name:           req.Name,
		Email:          req.Email,
		Role:           role,
		Status:         status,
		FaceDescriptor: string(descriptorJSON),
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания пользователя"})
		return
	}

	LogAction(c, user.ID, "register", "user", user.ID, "Регистрация: "+user.Email)

	// Первый пользователь (админ) — сразу входит
	if status == "approved" {
		tokenString, err := generateJWT(user.ID, user.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
			return
		}
		c.SetCookie("token", tokenString, 900, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{
			"message":  "Регистрация успешна",
			"user":     user.Name,
			"role":     user.Role,
			"approved": true,
		})
		return
	}

	// Остальные — ждут подтверждения
	c.JSON(http.StatusOK, gin.H{
		"message":  "Заявка отправлена. Ожидайте подтверждения администратором.",
		"user":     user.Name,
		"role":     user.Role,
		"approved": false,
	})
}

// GestureChallenge генерирует случайный жест для проверки при входе
func GestureChallenge(c *gin.Context) {
	gestures := []string{"thumbs_up", "peace", "open_palm", "one_finger"}
	gesture := gestures[rand.Intn(len(gestures))]

	// Создаём короткоживущий токен с жестом (2 минуты)
	claims := jwt.MapClaims{
		"gesture": gesture,
		"type":    "gesture_challenge",
		"exp":     time.Now().Add(2 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(middleware.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации задания"})
		return
	}

	labels := map[string]string{
		"thumbs_up":  "👍 Большой палец вверх",
		"peace":      "✌️ Знак мира (два пальца)",
		"open_palm":  "✋ Открытая ладонь",
		"one_finger": "☝️ Один палец вверх",
	}

	c.JSON(http.StatusOK, gin.H{
		"gesture": gesture,
		"label":   labels[gesture],
		"token":   tokenString,
	})
}

type LoginRequest struct {
	FaceDescriptor []float64 `json:"face_descriptor"`
	GestureToken   string    `json:"gesture_token"`
}

// Login обрабатывает вход по лицу
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if len(req.FaceDescriptor) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо отсканировать лицо"})
		return
	}

	// Проверяем токен жеста
	if req.GestureToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо пройти проверку жестом"})
		return
	}

	gestureToken, err := jwt.Parse(req.GestureToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return middleware.JWTSecret, nil
	})
	if err != nil || !gestureToken.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Проверка жеста не пройдена или истекла. Обновите страницу."})
		return
	}

	gestureClaims, ok := gestureToken.Claims.(jwt.MapClaims)
	if !ok || gestureClaims["type"] != "gesture_challenge" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недействительный токен жеста"})
		return
	}

	// Получаем всех пользователей
	var users []models.User
	database.DB.Find(&users)

	if len(users) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Нет зарегистрированных пользователей"})
		return
	}

	// Ищем совпадение
	var matchedUser *models.User
	minDistance := math.MaxFloat64
	threshold := 0.6

	for i := range users {
		if users[i].FaceDescriptor == "" {
			continue
		}

		var storedDescriptor []float64
		if err := json.Unmarshal([]byte(users[i].FaceDescriptor), &storedDescriptor); err != nil {
			continue
		}

		distance := euclideanDistance(req.FaceDescriptor, storedDescriptor)
		if distance < threshold && distance < minDistance {
			minDistance = distance
			matchedUser = &users[i]
		}
	}

	if matchedUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Лицо не распознано. Пожалуйста, зарегистрируйтесь."})
		return
	}

	// Проверяем статус пользователя
	if matchedUser.Status == "pending" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Ваша заявка ещё не подтверждена администратором."})
		return
	}
	if matchedUser.Status == "rejected" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Ваша заявка была отклонена администратором."})
		return
	}

	// Генерируем JWT
	tokenString, err := generateJWT(matchedUser.ID, matchedUser.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
		return
	}

	c.SetCookie("token", tokenString, 900, "/", "", false, true)

	LogAction(c, matchedUser.ID, "login", "user", matchedUser.ID, "Вход: "+matchedUser.Email)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Вход выполнен",
		"user":     matchedUser.Name,
		"role":     matchedUser.Role,
		"distance": minDistance,
	})
}

// Logout выполняет выход
func Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

// euclideanDistance вычисляет евклидово расстояние между двумя векторами
func euclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

// GestureVerifyChallenge генерирует задание жеста для подтверждения опасных действий
func GestureVerifyChallenge(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется аутентификация"})
		return
	}

	gestures := []string{"thumbs_up", "peace", "open_palm", "one_finger"}
	gesture := gestures[rand.Intn(len(gestures))]

	claims := jwt.MapClaims{
		"gesture": gesture,
		"type":    "gesture_reverify",
		"user_id": user.ID,
		"exp":     time.Now().Add(2 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(middleware.JWTSecret)

	labels := map[string]string{
		"thumbs_up":  "Большой палец вверх",
		"peace":      "Знак мира (два пальца)",
		"open_palm":  "Открытая ладонь",
		"one_finger": "Один палец вверх",
	}

	emojis := map[string]string{
		"thumbs_up":  "👍",
		"peace":      "✌️",
		"open_palm":  "✋",
		"one_finger": "☝️",
	}

	c.JSON(http.StatusOK, gin.H{
		"gesture": gesture,
		"label":   labels[gesture],
		"emoji":   emojis[gesture],
		"token":   tokenString,
	})
}

// ConfirmGestureVerification подтверждает прохождение жеста и выдаёт токен действия
func ConfirmGestureVerification(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется аутентификация"})
		return
	}

	var req struct {
		GestureToken string `json:"gesture_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат"})
		return
	}

	token, err := jwt.Parse(req.GestureToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return middleware.JWTSecret, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Проверка жеста не пройдена"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "gesture_reverify" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недействительный токен"})
		return
	}
	if uint(claims["user_id"].(float64)) != user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Токен для другого пользователя"})
		return
	}

	// Выдаём короткоживущий токен подтверждения (30 секунд)
	actionClaims := jwt.MapClaims{
		"type":    "action_confirmed",
		"user_id": user.ID,
		"exp":     time.Now().Add(30 * time.Second).Unix(),
	}
	actionToken := jwt.NewWithClaims(jwt.SigningMethodHS256, actionClaims)
	actionTokenString, _ := actionToken.SignedString(middleware.JWTSecret)

	c.JSON(http.StatusOK, gin.H{"action_token": actionTokenString})
}

// generateJWT генерирует JWT-токен
func generateJWT(userID uint, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(middleware.JWTSecret)
}
