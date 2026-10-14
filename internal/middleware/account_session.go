package middleware

import (
	"errors"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountSession verifica cuentas vigentes incluso en rutas que leen la cookie directamente.
func AccountSession(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return accountSession(cfg, func(id uuid.UUID) (*repository.AccountProfile, error) {
		return repository.GetAccountProfile(pool, id)
	})
}

func accountSession(cfg *config.Config, lookup func(uuid.UUID) (*repository.AccountProfile, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Login, recuperación e invitaciones no dependen de la sesión anterior.
		if strings.HasPrefix(c.Request.URL.Path, "/auth/") && c.Request.URL.Path != "/auth/me" {
			c.Next()
			return
		}
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			c.Next()
			return
		}
		if !setSessionClaims(c, cfg.JWTSecret, token) {
			return
		}
		id, _ := uuid.Parse(c.GetString("user_id"))
		profile, err := lookup(id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !profile.Active) {
			c.SetSameSite(http.SameSiteNoneMode)
			c.SetCookie("session_token", "", -1, "/", "", true, true)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "la cuenta ya no está disponible"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "no se pudo verificar la cuenta"})
			return
		}
		c.Next()
	}
}
