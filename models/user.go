package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Name           string         `gorm:"size:255;not null" json:"name"`
	Email          string         `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Role           string         `gorm:"size:50;not null;default:'viewer'" json:"role"`   // admin, director, accountant, manager, engineer, viewer
	Status         string         `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, approved, rejected
	FaceDescriptor string         `gorm:"type:text" json:"face_descriptor"`
	FaceLandmarks  string         `gorm:"type:text" json:"face_landmarks"` // 68 точек лица для геометрического метода
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

func (u *User) IsManager() bool {
	return u.Role == "manager"
}

func (u *User) IsDirector() bool {
	return u.Role == "director"
}

func (u *User) CanEdit() bool {
	return u.Role == "admin" || u.Role == "director" || u.Role == "manager"
}

func (u *User) IsApproved() bool {
	return u.Status == "approved"
}

func (u *User) IsPending() bool {
	return u.Status == "pending"
}

// RoleLabel возвращает русское название роли
func RoleLabel(role string) string {
	labels := map[string]string{
		"admin":      "Администратор",
		"director":   "Директор",
		"accountant": "Бухгалтер",
		"manager":    "Менеджер",
		"engineer":   "Инженер",
		"viewer":     "Зритель",
	}
	if label, ok := labels[role]; ok {
		return label
	}
	return role
}

// StatusLabel возвращает русское название статуса
func StatusLabelUser(status string) string {
	labels := map[string]string{
		"pending":  "Ожидание",
		"approved": "Одобрен",
		"rejected": "Отклонён",
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}
