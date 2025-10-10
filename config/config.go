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
type ServerConfig struct {
	PORT         string
	IMAP_API_KEY string
}
type IMAPConfig struct {
	IMAP_PORT     string
	TLS_CERT_FILE string
	TLS_KEY_FILE  string
}
type Config struct {
	DB   DBConfig
	API  ServerConfig
	IMAP IMAPConfig
}
type DirectoryConfig struct {
	DataDir      string
	CertDir      string
	CertFile     string
	KeyFile      string
	GluonDataDir string
	GluonDBPath  string
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
		API: ServerConfig{
			PORT:         os.Getenv("PORT"),
			IMAP_API_KEY: os.Getenv("IMAP_API_KEY"),
		},
		IMAP: IMAPConfig{
			IMAP_PORT:     os.Getenv("IMAP_PORT"),
			TLS_CERT_FILE: os.Getenv("TLS_CERT_FILE"),
			TLS_KEY_FILE:  os.Getenv("TLS_KEY_FILE"),
		},
	}
	log.Println("✅ Config loaded")
	return cfg
}

// getEnv returns the value of the environment variable named by the key.
// If the variable is not set, it returns the fallback value.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
