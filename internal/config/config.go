package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// App
	AppPort string
	AppEnv  string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// JWT
	JWTSecret      string
	JWTExpireHours int

	// CORS
	CORSOrigins string

	// SMTP
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string

	// Tracking
	TrackingBaseURL string
}

// Load reads configuration from .env file and environment variables.
func Load() *Config {
	// Load .env file (ignore error if not found — env vars may be set directly)
	_ = godotenv.Load()

	return &Config{
		// App
		AppPort: getEnv("APP_PORT", "3001"),
		AppEnv:  getEnv("APP_ENV", "development"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "realsend_user"),
		DBPassword: getEnv("DB_PASSWORD", "realsend_secret"),
		DBName:     getEnv("DB_NAME", "realsend"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Redis
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		// JWT
		JWTSecret:      getEnv("JWT_SECRET", "change-me"),
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 24),

		// CORS
		CORSOrigins: getEnv("CORS_ORIGINS", "http://localhost:3000"),

		// SMTP
		SMTPHost:      getEnv("SMTP_HOST", "localhost"),
		SMTPPort:      getEnv("SMTP_PORT", "1025"),
		SMTPUsername:  getEnv("SMTP_USERNAME", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFromName:  getEnv("SMTP_FROM_NAME", "RealSend"),
		SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", "noreply@realsend.id"),

		// Tracking
		TrackingBaseURL: getEnv("TRACKING_BASE_URL", "http://localhost:3001"),
	}
}

// DatabaseURL returns the PostgreSQL connection string.
func (c *Config) DatabaseURL() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword +
		"@" + c.DBHost + ":" + c.DBPort +
		"/" + c.DBName + "?sslmode=" + c.DBSSLMode
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// getEnv reads an env variable with a fallback default.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// getEnvInt reads an env variable as int with a fallback default.
func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
