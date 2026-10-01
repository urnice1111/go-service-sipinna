package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	Port          string
	JWTSecret     string
	SipinnaBucket string
	FrontendURL   string
	// API key de TypeSafe para el análisis de reportes con Jev (opcional)
	TypeSafeAPIKey string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var config *Config = &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           os.Getenv("PORT"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		SipinnaBucket:  os.Getenv("SIPINNA_BUCKET"),
		FrontendURL:    os.Getenv("FRONTEND_URL"),
		TypeSafeAPIKey: os.Getenv("TYPESAFE_API_KEY"),
	}

	if strings.TrimSpace(config.JWTSecret) == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return config, nil

}
