package middleware

import (
	"errors"
	"go-service-sipinna/internal/config"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrJWTNotConfigured = errors.New("JWT secret is not configured")

// AuthRequired es igual que [AuthMiddleware], pero lee el secreto directamente de la
// variable de entorno JWT_SECRET en lugar de recibir la configuración.
func AuthRequired() gin.HandlerFunc {
	return authRequired(os.Getenv("JWT_SECRET"))
}

// AuthMiddleware exige una sesión válida en la cookie "session_token".
//
// Si el token es válido, deja en el contexto de Gin:
//   - "user": todos los claims ([jwt.MapClaims])
//   - "user_id": id del usuario (string con un UUID)
//   - "is_admin": true si es personal con cuenta activada (bool)
//   - "user_type": "citizen", "administrador" o "alimentador" (string)
//
// Si falta la cookie o el token no es válido, aborta con 401. Si cfg es nil o no
// tiene JWTSecret, aborta con 500.
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	if cfg == nil {
		return authRequired("")
	}
	return authRequired(cfg.JWTSecret)
}

// OptionalAuth deja el usuario de la sesión en el contexto cuando hay cookie.
// Sin cookie continúa como anónimo. Una cookie inválida responde 401.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("session_token")
		if err != nil || tokenStr == "" {
			c.Next()
			return
		}

		if !setSessionClaims(c, os.Getenv("JWT_SECRET"), tokenStr) {
			return
		}
		c.Next()
	}
}

// authRequired construye el middleware de autenticación con el secreto indicado.
func authRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("session_token")
		if err != nil || tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if !setSessionClaims(c, jwtSecret, tokenStr) {
			return
		}
		c.Next()
	}
}

// setSessionClaims valida tokenStr y, si es válido, deja en el contexto los mismos
// valores que [AuthMiddleware]. Si falla, la respuesta de error ya se escribió.
func setSessionClaims(c *gin.Context, jwtSecret, tokenStr string) bool {
	if strings.TrimSpace(jwtSecret) == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": ErrJWTNotConfigured.Error()})
		return false
	}

	claims, err := validateJWT(tokenStr, jwtSecret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return false
	}

	c.Set("user", claims)

	userID, ok := claims["user_id"].(string)
	id, err := uuid.Parse(userID)
	if !ok || err != nil || id == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
		return false
	}
	c.Set("user_id", id.String())

	isAdmin, ok := claims["is_admin"].(bool)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "is_admin missing or invalid"})
		return false
	}
	c.Set("is_admin", isAdmin)

	userType, ok := claims["user_type"].(string)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_type missing or invalid"})
		return false
	}
	c.Set("user_type", userType)

	return true
}

// UserIDFromRequest lee el usuario de la cookie "session_token" sin exigir que el
// middleware de autenticación haya corrido. Regresa [ErrUnauthorized] si no hay sesión
// válida y [ErrJWTNotConfigured] si falta JWT_SECRET.
func UserIDFromRequest(c *gin.Context) (uuid.UUID, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(jwtSecret) == "" {
		return uuid.Nil, ErrJWTNotConfigured
	}

	tokenStr, err := c.Cookie("session_token")
	if err != nil || tokenStr == "" {
		return uuid.Nil, ErrUnauthorized
	}

	claims, err := validateJWT(tokenStr, jwtSecret)
	if err != nil {
		return uuid.Nil, ErrUnauthorized
	}

	userID, ok := claims["user_id"].(string)
	id, parseErr := uuid.Parse(userID)
	if !ok || parseErr != nil || id == uuid.Nil {
		return uuid.Nil, ErrUnauthorized
	}

	return id, nil
}

// validateJWT verifica la firma HS256 de tokenStr con jwtSecret y que el token tenga
// una expiración ("exp") vigente. Regresa los claims si el token es válido.
func validateJWT(tokenStr, jwtSecret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		return []byte(jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
