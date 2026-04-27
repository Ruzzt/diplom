package models

import "time"

type ProjectAssignment struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProjectID  uint      `gorm:"uniqueIndex:idx_project_user;not null" json:"project_id"`
	Project    Project   `gorm:"foreignKey:ProjectID" json:"project"`
	UserID     uint      `gorm:"uniqueIndex:idx_project_user;not null" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	AssignedBy uint      `json:"assigned_by"`
	CreatedAt  time.Time `json:"created_at"`
}
