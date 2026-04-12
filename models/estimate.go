package models

import (
	"time"

	"gorm.io/gorm"
)

type Estimate struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ProjectID   uint           `gorm:"not null" json:"project_id"`
	Project     Project        `gorm:"foreignKey:ProjectID" json:"-"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	TotalAmount float64        `gorm:"type:decimal(15,2);default:0" json:"total_amount"`
	Items       []EstimateItem `gorm:"foreignKey:EstimateID" json:"items"`
	CreatedByID uint           `json:"created_by_id"`
	CreatedBy   User           `gorm:"foreignKey:CreatedByID" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type EstimateItem struct {
	ID         uint    `gorm:"primaryKey" json:"id"`
	EstimateID uint    `gorm:"not null" json:"estimate_id"`
	Name       string  `gorm:"size:255;not null" json:"name"`
	Unit       string  `gorm:"size:50" json:"unit"` // шт, м, м2, м3, кг, т
	Quantity   float64 `gorm:"type:decimal(10,3);default:0" json:"quantity"`
	UnitPrice  float64 `gorm:"type:decimal(15,2);default:0" json:"unit_price"`
	TotalPrice float64 `gorm:"type:decimal(15,2);default:0" json:"total_price"`
}

func Units() []string {
	return []string{"шт", "м", "м²", "м³", "кг", "т", "л", "компл"}
}
