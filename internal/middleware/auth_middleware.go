package middleware

import (
	"go-service-sipinna/internal/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(cfg.JWTSecret) == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
			return
		}

		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer <token>"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
			return []byte(cfg.JWTSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}
		userID, ok := claims["user_id"].(string)
		id, err := uuid.Parse(userID)
		if !ok || err != nil || id == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			return
		}

		c.Set("user_id", id.String())
		c.Next()
	}
}
