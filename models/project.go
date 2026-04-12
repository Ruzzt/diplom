package models

import (
	"time"

	"gorm.io/gorm"
)

type Project struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Status      string         `gorm:"size:50;not null;default:'planning'" json:"status"` // planning, active, completed, suspended
	Address     string         `gorm:"size:500" json:"address"`
	StartDate   *time.Time     `json:"start_date"`
	EndDate     *time.Time     `json:"end_date"`
	Budget      float64        `gorm:"type:decimal(15,2);default:0" json:"budget"`
	CreatedByID uint           `json:"created_by_id"`
	CreatedBy   User           `gorm:"foreignKey:CreatedByID" json:"created_by"`
	Documents   []Document     `gorm:"foreignKey:ProjectID" json:"documents"`
	Estimates   []Estimate     `gorm:"foreignKey:ProjectID" json:"estimates"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func ProjectStatuses() []string {
	return []string{"planning", "active", "completed", "suspended"}
}

func ProjectStatusLabel(status string) string {
	labels := map[string]string{
		"planning":  "Планирование",
		"active":    "В работе",
		"completed": "Завершён",
		"suspended": "Приостановлен",
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}
