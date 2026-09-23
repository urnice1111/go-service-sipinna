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

func AuthRequired() gin.HandlerFunc {
	return authRequired(os.Getenv("JWT_SECRET"))
}

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	if cfg == nil {
		return authRequired("")
	}
	return authRequired(cfg.JWTSecret)
}

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
