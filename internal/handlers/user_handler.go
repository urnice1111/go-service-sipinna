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
	Email    *string `json:"email"`
	Phone    *string `json:"number"`
	Password string  `json:"password" binding:"required"`
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

		respondWithToken(c, http.StatusCreated, newUser.ID, cfg, false, "citizen", "change_later")

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

		respondWithToken(c, http.StatusCreated, newUser.ID, cfg, true, req.Role, "change_later")

	}

}

func AcceptOrRejectAdmin(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !c.GetBool("is_admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin privileges are required"})
			return
		}
		adminID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid admin id",
			})
			return
		}

		accountStatus, err := repository.ActivateAdminAccount(
			pool,
			adminID,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "admin not found or account is already deactivated",
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not update account status",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":            adminID,
			"estado_cuenta": accountStatus,
		})
	}
}

func LoginHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		if (req.Email == nil) == (req.Phone == nil) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provide either email or number, not both/neither"})
			return
		}

		user, err := repository.GetUserByContact(pool, stringValue(req.Email), stringValue(req.Phone))
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
			return
		}

		// I dunno why i did this, maybe drunk but change later to fetch insnant on previous query
		userType, isAdmin, err := repository.GetUserType(pool, user.ID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user type"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		respondWithToken(c, http.StatusOK, user.ID, cfg, isAdmin, userType, user.Name)
	}
}

func LogoutHandler(c *gin.Context) {
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("session_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

/*HELPERS*/

func respondWithToken(c *gin.Context, status int, userID uuid.UUID, cfg *config.Config, isAdmin bool, userType string, userName string) {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
		return
	}

	claims := jwt.MapClaims{
		"user_id":   userID.String(),
		"exp":       time.Now().Add(7 * 24 * time.Hour).Unix(),
		"is_admin":  isAdmin,
		"user_type": userType,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	const maxAge = 60 * 60 * 24 * 7
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("session_token", token, maxAge, "/", "", true, true)

	c.JSON(status, gin.H{"name": userName})
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil
	}

	return &value
}
