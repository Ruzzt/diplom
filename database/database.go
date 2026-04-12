package database

import (
	"face-auth-system/config"
	"face-auth-system/models"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(cfg *config.Config) {
	var err error
	DB, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatal("Не удалось подключиться к базе данных: ", err)
	}
	log.Println("Подключение к базе данных установлено")
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.Document{},
		&models.Estimate{},
		&models.EstimateItem{},
	)
	if err != nil {
		log.Fatal("Ошибка миграции: ", err)
	}
	log.Println("Миграция базы данных выполнена")
}
