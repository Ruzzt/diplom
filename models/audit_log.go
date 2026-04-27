package models

import "time"

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"user"`
	Action     string    `gorm:"size:100;not null;index" json:"action"`
	TargetType string    `gorm:"size:50" json:"target_type"`
	TargetID   uint      `json:"target_id"`
	Details    string    `gorm:"type:text" json:"details"`
	IP         string    `gorm:"size:45" json:"ip"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func ActionLabel(action string) string {
	labels := map[string]string{
		"login":          "Вход в систему",
		"register":       "Регистрация",
		"create_project": "Создание проекта",
		"update_project": "Изменение проекта",
		"delete_project": "Удаление проекта",
		"upload_document": "Загрузка документа",
		"delete_document": "Удаление документа",
		"create_estimate": "Создание сметы",
		"update_estimate": "Изменение сметы",
		"delete_estimate": "Удаление сметы",
		"change_role":     "Изменение роли",
		"approve_user":    "Одобрение пользователя",
		"reject_user":     "Отклонение пользователя",
		"delete_user":     "Удаление пользователя",
		"assign_user":     "Назначение на проект",
		"unassign_user":   "Снятие с проекта",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return action
}
