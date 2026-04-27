package middleware

import (
	"github.com/golang-jwt/jwt/v5"
)

// ValidateActionToken проверяет токен подтверждения жестом
func ValidateActionToken(tokenString string, userID uint) bool {
	if tokenString == "" {
		return false
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return JWTSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "action_confirmed" {
		return false
	}
	if uint(claims["user_id"].(float64)) != userID {
		return false
	}
	return true
}
