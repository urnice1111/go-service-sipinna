package handlers

import (
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type CreateCitizen struct {
	Name            string `json:"nombre" binding:"required"`
	Role            string `json:"rol"`
	Age             int    `json:"edad"`
	Genre           string `json:"genero"`
	Email           string `json:"email" binding:"required_without=TelephoneNumber,omitempty,email"`
	TelephoneNumber string `json:"telefono" binding:"required_without=Email,omitempty,e164"`
	Password        string `json:"password" binding:"required"`
}

func CitizenSignInHandler(pool *pgxpool.Pool) gin.HandlerFunc {
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
		}

		user := &models.User{
			ID:              idV7,
			Name:            req.Name,
			Role:            "citizen",
			Age:             req.Age,
			Genre:           req.Genre,
			Email:           optionalString(req.Email),
			TelephoneNumber: optionalString(req.TelephoneNumber),
			HashedPassword:  string(HashedPassword),
			AccountState:    "active",
		}

		newUser, err := repository.CreateUser(pool, user)

		//TODO :- Add logic to verify what type of error is: if the user already exists etc
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error registering the user on the db" + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, newUser)

	}

}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil
	}

	return &value
}
