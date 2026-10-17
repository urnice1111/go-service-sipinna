package handlers

import (
	"errors"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// CreateCitizen es el cuerpo de POST /auth/citizen. Se requiere correo o teléfono
type CreateCitizen struct {
	Name            string `json:"nombre" binding:"required"`
	Age             int    `json:"edad"`
	Genre           string `json:"genero"`
	Email           string `json:"email" binding:"required_without=TelephoneNumber,omitempty,email"`
	TelephoneNumber string `json:"telefono" binding:"required_without=Email"`
	Password        string `json:"password" binding:"required"`
}

var citizenPhonePattern = regexp.MustCompile(`^(?:[0-9]{10}|\+[1-9][0-9]{1,14})$`)

// LoginRequest es el cuerpo de POST /auth/login. Debe traer exactamente uno de
// Email o Phone.
type LoginRequest struct {
	Email    *string `json:"email"`
	Phone    *string `json:"number"`
	Password string  `json:"password" binding:"required"`
}

// CitizenSignInHandler maneja POST /auth/citizen: registra un ciudadano con contraseña
// (debe cumplir las reglas de validatePassword, guardada con bcrypt) e inicia su sesión.
// Responde 201 y deja la cookie de sesión.
func CitizenSignInHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateCitizen
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"erroruwu": err.Error()})
			return
		}

		emailMissing := strings.TrimSpace(req.Email) == ""
		phoneMissing := strings.TrimSpace(req.TelephoneNumber) == ""
		if !phoneMissing && !citizenPhonePattern.MatchString(req.TelephoneNumber) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El número telefónico no es correcto."})
			return
		}

		if emailMissing && phoneMissing {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "email or telephone number is required",
			})
			return
		}

		// Revisa las reglas de la contraseña antes de hashearla
		if msg := validatePassword(req.Password); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
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

		respondWithToken(c, http.StatusCreated, newUser.ID, cfg, false, "citizen", newUser.Name, "")

	}
}

// LoginHandler maneja POST /auth/login: valida correo o teléfono y contraseña, y si
// son correctos deja la cookie de sesión. Responde 401 con el mismo mensaje tanto si
// el usuario no existe como si la contraseña es incorrecta.
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
		userType, isAdmin, zoneName, err := repository.GetUserType(pool, user.ID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user type"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		respondWithToken(c, http.StatusOK, user.ID, cfg, isAdmin, userType, user.Name, zoneName)
	}
}

// LogoutHandler maneja POST /auth/logout: borra la cookie de sesión.
func LogoutHandler(c *gin.Context) {
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("session_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

/*HELPERS*/

// respondWithToken firma un JWT HS256 con vigencia de 7 días, lo guarda en la cookie
// "session_token" (HttpOnly, Secure, SameSite=None) y responde status con el nombre,
// tipo de usuario y zona. Responde 403 sin emitir sesión si es personal cuya cuenta
// no está activada.
func respondWithToken(c *gin.Context, status int, userID uuid.UUID, cfg *config.Config, isAdmin bool, userType string, userName string, zoneName string) {
	if userType != "citizen" && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "La cuenta aún no está activada."})
		return
	}

	if strings.TrimSpace(cfg.JWTSecret) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
		return
	}

	claims := jwt.MapClaims{
		"user_id":   userID.String(),
		"exp":       time.Now().Add(7 * 24 * time.Hour).Unix(),
		"is_admin":  isAdmin,
		"user_type": userType,
		"name":      userName,
		"zone_name": zoneName,
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	const maxAge = 60 * 60 * 24 * 7
	c.SetSameSite(http.SameSiteNoneMode)
	c.SetCookie("session_token", token, maxAge, "/", "", true, true)

	c.JSON(status, gin.H{"name": userName, "user_type": userType, "zone_name": zoneName})
}

// stringValue regresa el valor de value sin espacios en los extremos, o "" si es nil.
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// optionalString regresa nil si value está vacío (o solo tiene espacios); si no,
// un puntero al valor recortado.
func optionalString(value string) *string {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil
	}

	return &value
}
