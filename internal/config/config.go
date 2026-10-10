package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config agrupa la configuración del servicio. Cada campo sale de la variable de
// entorno indicada.
type Config struct {
	// Cadena de conexión de PostgreSQL (DATABASE_URL)
	DatabaseURL string
	// Puerto HTTP en el que escucha el servidor (PORT)
	Port string
	// Secreto HMAC para firmar y validar los JWT de sesión (JWT_SECRET, obligatorio)
	JWTSecret string
	// Bucket de S3 donde se guardan las fotos de los reportes (SIPINNA_BUCKET)
	SipinnaBucket string
	// Origen adicional permitido por CORS (FRONTEND_URL, opcional)
	FrontendURL string
	// API key de TypeSafe para el análisis de reportes con Jev (opcional)
	TypeSafeAPIKey string
	// API key de Resend para enviar correos (SIPINNA_RESEND_API_KEY)
	SipinnaResendAPIKey string
	// Proyecto de Supabase usado solo para validar los inicios de sesión con Google
	SupabaseURL     string
	SupabaseAnonKey string
}

// Load lee la configuración de las variables de entorno. Si existe un archivo .env
// lo carga primero; si no existe, se ignora sin error.
//
// SUPABASE_URL se normaliza sin la diagonal final. Regresa un error si JWT_SECRET
// no está definido, porque sin él no se pueden emitir ni validar sesiones.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var config *Config = &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                os.Getenv("PORT"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		SipinnaBucket:       os.Getenv("SIPINNA_BUCKET"),
		FrontendURL:         os.Getenv("FRONTEND_URL"),
		TypeSafeAPIKey:      os.Getenv("TYPESAFE_API_KEY"),
		SipinnaResendAPIKey: os.Getenv("SIPINNA_RESEND_API_KEY"),
		SupabaseURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/"),
		SupabaseAnonKey:     strings.TrimSpace(os.Getenv("SUPABASE_ANON_KEY")),
	}

	if strings.TrimSpace(config.JWTSecret) == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	return config, nil

}
