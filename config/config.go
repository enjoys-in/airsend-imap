package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

type DBConfig struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}
type Config struct {
	DB DBConfig
}

func GetConfig() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	cfg := Config{
		DB: DBConfig{
			DBHost:     os.Getenv("DB_HOST"),
			DBPort:     os.Getenv("DB_PORT"),
			DBUser:     os.Getenv("DB_USER"),
			DBPassword: os.Getenv("DB_PASSWORD"),
			DBName:     os.Getenv("DB_NAME"),
			DBSSLMode:  os.Getenv("DB_SSLMODE"),
		},
	}
	log.Println("✅ Config loaded")
	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
