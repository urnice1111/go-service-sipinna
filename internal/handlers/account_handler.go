package handlers

import (
	"errors"
	"go-service-sipinna/internal/repository"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var profilePhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)

type UpdateProfileRequest struct {
	Name   string `json:"nombre" binding:"required,max=200"`
	Email  string `json:"email" binding:"required_without=Phone,omitempty,email,max=254"`
	Phone  string `json:"telefono" binding:"required_without=Email,omitempty,e164"`
	Age    *int   `json:"edad" binding:"required,gte=0,lte=120"`
	Gender string `json:"genero" binding:"omitempty,oneof=mujer hombre otro prefiero-no-decir"`
}

func GetProfileHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := uuid.Parse(c.GetString("user_id"))
		profile, err := repository.GetAccountProfile(pool, id)
		if err != nil {
			accountError(c, err)
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}

func UpdateProfileHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("user_type") != "citizen" {
			c.JSON(http.StatusForbidden, gin.H{"error": "solo el ciudadano puede editar su perfil"})
			return
		}
		if c.ContentType() != "application/json" {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "se requiere JSON"})
			return
		}
		var req UpdateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "revisa el nombre, edad, género y datos de contacto (teléfono con prefijo internacional)"})
			return
		}

		if req.Phone != "" && !profilePhonePattern.MatchString(req.Phone) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el teléfono debe incluir + y el código del país, sin espacios"})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el nombre es obligatorio"})
			return
		}
		id, _ := uuid.Parse(c.GetString("user_id"))
		profile, err := repository.UpdateCitizenProfile(pool, id, repository.AccountProfile{
			Name: req.Name, Email: optionalString(strings.ToLower(req.Email)), Phone: optionalString(req.Phone), Age: *req.Age, Gender: req.Gender,
		})
		if err != nil {
			accountError(c, err)
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}

func DeleteAccountHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.ContentType() != "application/json" {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "se requiere JSON"})
			return
		}
		var req struct {
			Confirmation string `json:"confirmation"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Confirmation != "ELIMINAR" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "escribe ELIMINAR para confirmar"})
			return
		}
		// El ID siempre proviene de la sesión: nadie puede eliminar la cuenta de otro usuario.
		id, _ := uuid.Parse(c.GetString("user_id"))
		if err := repository.DeleteAccount(pool, id); err != nil {
			accountError(c, err)
			return
		}
		LogoutHandler(c)
	}
}

func accountError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada"})
	case errors.Is(err, repository.ErrGoogleEmail):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		c.JSON(http.StatusConflict, gin.H{"error": "ya existe una cuenta con ese correo o teléfono"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo completar la operación de la cuenta"})
	}
}

// MeHandler carga la sesión desde la BD para reflejar cambios del perfil y del rol.
func MeHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := uuid.Parse(c.GetString("user_id"))
		profile, err := repository.GetAccountProfile(pool, id)
		if err != nil {
			accountError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": profile.Name, "user_type": profile.UserType, "zone_name": profile.ZoneName})
	}
}
