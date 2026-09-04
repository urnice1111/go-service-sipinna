package handlers

import (
	"errors"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type CreateCitizen struct {
	Name            string `json:"nombre" binding:"required"`
	Age             int    `json:"edad"`
	Genre           string `json:"genero"`
	Email           string `json:"email" binding:"required_without=TelephoneNumber,omitempty,email"`
	TelephoneNumber string `json:"telefono" binding:"required_without=Email,omitempty,e164"`
	Password        string `json:"password" binding:"required"`
}

type CreateAdmin struct {
	Name            string `json:"nombre" binding:"required"`
	Role            string `json:"rol" biding:"required"`
	Email           string `json:"email" binding:"required_without=TelephoneNumber,omitempty,email"`
	TelephoneNumber string `json:"telefono" binding:"required_without=Email,omitempty,e164"`
	Password        string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email           string `json:"email" binding:"required_without=TelephoneNumber,excluded_with=TelephoneNumber,omitempty,email"`
	TelephoneNumber string `json:"telefono" binding:"required_without=Email,excluded_with=Email,omitempty,e164"`
	Password        string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token  string    `json:"token"`
	UserID uuid.UUID `json:"user_id"`
}

func CitizenSignInHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateCitizen
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"erroruwu": err.Error()})
			return
		}

		emailMissing := strings.TrimSpace(req.Email) == ""
		phoneMissing := strings.TrimSpace(req.TelephoneNumber) == ""

		if emailMissing && phoneMissing {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "email or telephone number is required",
			})
			return
		}

		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6"})
			return
		}

		var HashedPassword, err = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password" + err.Error()})
			return
		}

		idV7, err := uuid.NewV7()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate uuid" + err.Error()})
			return
		}

		user := &models.User{
			ID:              idV7,
			Name:            req.Name,
			Age:             req.Age,
			Genre:           req.Genre,
			Email:           optionalString(req.Email),
			TelephoneNumber: optionalString(req.TelephoneNumber),
			HashedPassword:  string(HashedPassword),
		}

		newUser, err := repository.CreateUser(pool, user)

		//TODO :- Add logic to verify what type of error is: if the user already exists etc
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering the user on the db" + err.Error()})
			return
		}

		respondWithToken(c, http.StatusCreated, newUser.ID, cfg, false)

	}
}

func AdminSignInHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAdmin
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		emailMissing := strings.TrimSpace(req.Email) == ""
		phoneMissing := strings.TrimSpace(req.TelephoneNumber) == ""

		if emailMissing && phoneMissing {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "email or telephone number is required",
			})
			return
		}

		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6"})
			return
		}

		var HashedPassword, err = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password" + err.Error()})
			return
		}

		idV7, err := uuid.NewV7()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate uuid" + err.Error()})
			return
		}

		user := &models.User{
			ID:              idV7,
			Name:            req.Name,
			Role:            req.Role,
			Email:           optionalString(req.Email),
			TelephoneNumber: optionalString(req.TelephoneNumber),
			HashedPassword:  string(HashedPassword),
		}

		newUser, err := repository.CreateAdmin(pool, user)

		//TODO :- Add logic to verify what type of error is: if the user already exists etc
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering the user on the db" + err.Error()})
			return
		}

		respondWithToken(c, http.StatusCreated, newUser.ID, cfg, false)

	}

}

func LoginHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := repository.GetUserByContact(pool, req.Email, req.TelephoneNumber)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		respondWithToken(c, http.StatusOK, user.ID, cfg, false)
	}
}

/*HELPERS*/

func respondWithToken(c *gin.Context, status int, userID uuid.UUID, cfg *config.Config, isAdmin bool) {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
		return
	}

	claims := jwt.MapClaims{
		"user_id":  userID.String(),
		"exp":      time.Now().Add(time.Hour).Unix(),
		"is_admin": isAdmin,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(status, AuthResponse{Token: token, UserID: userID})
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil
	}

	return &value
}
