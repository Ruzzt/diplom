package models

import (
	"time"

	"gorm.io/gorm"
)

type Document struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ProjectID    *uint          `json:"project_id"`
	Project      Project        `gorm:"foreignKey:ProjectID" json:"-"`
	Name         string         `gorm:"size:255;not null" json:"name"`
	FilePath     string         `gorm:"size:500;not null" json:"file_path"`
	DocType      string         `gorm:"size:100" json:"doc_type"` // blueprint, permit, contract, report, other
	FileSize     int64          `json:"file_size"`
	UploadedByID uint           `json:"uploaded_by_id"`
	UploadedBy   User           `gorm:"foreignKey:UploadedByID" json:"uploaded_by"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (d *Document) IsCorporate() bool {
	return d.ProjectID == nil || *d.ProjectID == 0
}

func DocTypes() []string {
	return []string{"blueprint", "permit", "contract", "report", "other"}
}

func DocTypeLabel(docType string) string {
	labels := map[string]string{
		"blueprint": "Чертёж",
		"permit":    "Разрешение",
		"contract":  "Договор",
		"report":    "Отчёт",
		"other":     "Прочее",
	}
	if label, ok := labels[docType]; ok {
		return label
	}
	return docType
}
