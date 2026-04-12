package config

import "os"

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string
	ServerPort string
}

func Load() *Config {
	return &Config{
		DBHost:     getEnv("DB_HOST", "/tmp"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "tatanaruzavina"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "face_auth_db"),
		JWTSecret:  getEnv("JWT_SECRET", "diploma-secret-key-2024"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}
}

func (c *Config) DSN() string {
	dsn := "host=" + c.DBHost +
		" user=" + c.DBUser +
		" dbname=" + c.DBName +
		" port=" + c.DBPort +
		" sslmode=disable"
	if c.DBPassword != "" {
		dsn += " password=" + c.DBPassword
	}
	return dsn
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
