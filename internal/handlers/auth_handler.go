package handlers

import (
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// POST /login
func LoginHandler(pool *pgxpool.Pool, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := repository.FindUserByEmail(pool, strings.TrimSpace(req.Email))

		// Mismo mensaje si el usuario no existe o si la contraseña esta mal,
		// para no revelar cual de los dos fue
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciales incorrectas"})
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciales incorrectas"})
			return
		}

		if user.AccountState != "active" {
			c.JSON(http.StatusForbidden, gin.H{"error": "la cuenta no esta activa"})
			return
		}

		token, err := generateToken(user, jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token" + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"usuario": gin.H{
				"id":     user.ID,
				"nombre": user.Name,
				"email":  user.Email,
				"rol":    user.Role,
			},
		})
	}
}

// Crea el JWT con el id y rol del usuario. Dura 7 dias.
func generateToken(user *models.User, secret string) (string, error) {
	var claims jwt.MapClaims = jwt.MapClaims{
		"sub": user.ID.String(),
		"rol": user.Role,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}

	var token *jwt.Token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}
