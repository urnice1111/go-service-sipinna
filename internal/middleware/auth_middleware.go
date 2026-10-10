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

// authRequired construye el middleware de autenticación con el secreto indicado.
func authRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(jwtSecret) == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
			return
		}

		tokenStr, err := c.Cookie("session_token")
		if err != nil || tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims, err := validateJWT(tokenStr, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set("user", claims)

		userID, ok := claims["user_id"].(string)
		id, err := uuid.Parse(userID)
		if !ok || err != nil || id == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			return
		}
		c.Set("user_id", id.String())

		isAdmin, ok := claims["is_admin"].(bool)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "is_admin missing or invalid"})
			return
		}
		c.Set("is_admin", isAdmin)

		userType, ok := claims["user_type"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user_type missing or invalid"})
			return
		}
		c.Set("user_type", userType)

		c.Next()
	}
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
