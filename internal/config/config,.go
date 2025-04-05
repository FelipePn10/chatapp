package config

import (
	"os"

	"github.com/rs/zerolog"
)

type Config struct {
	Port       string
	DBUser     string
	DBPassword string
	DBHost     string
	DBName     string
	JWTSecret  string
	LogLevel   string
}

func LoadConfig() *Config {
	return &Config{
		Port:       getEnv("PORT", "8081"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBName:     getEnv("DB_NAME", "chat_db"),
		JWTSecret:  getEnv("JWT_SECRET", "default-secret-key"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

type Logger struct {
	zerolog.Logger
}

func SetupLogger(cfg *Config) *Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger := zerolog.New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Logger()

	return &Logger{logger}
}
