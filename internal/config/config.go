package config

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	Port          string
	JWTSecret     string
	SipinnaBucket string
}

func Load() (*Config, error) {
	var err error = godotenv.Load()
	if err != nil {
		log.Println("Warning: .env not found")
	}

	var config *Config = &Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Port:          os.Getenv("PORT"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		SipinnaBucket: os.Getenv("SIPINNA_BUCKET"),
	}

	if strings.TrimSpace(config.JWTSecret) == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return config, nil

}
