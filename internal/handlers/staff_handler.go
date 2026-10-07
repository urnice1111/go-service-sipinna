package handlers

import (
	"errors"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type CreateStaffRequest struct {
	Name            string  `json:"nombre" binding:"required"`
	Role            string  `json:"rol" binding:"required"`
	ZoneID          *string `json:"zona_id"`
	Email           string  `json:"email" binding:"omitempty,email"`
	TelephoneNumber string  `json:"telefono" binding:"omitempty,e164"`
	Password        string  `json:"password" binding:"required"`
	// The account created by an admin is active
	Activate *bool `json:"activar"`
}

// Fields ommited keeps their actual values
type UpdateStaffRequest struct {
	Role         *string `json:"rol"`
	ZoneID       *string `json:"zona_id"`
	AccountState *string `json:"estado_cuenta"`
}

// requireAdministrador allows only activated 'administrador' accounts (not 'alimentador').
func requireAdministrador(c *gin.Context, pool *pgxpool.Pool) (uuid.UUID, bool) {
	userID, scopeZoneID, ok := requireActiveStaff(c, pool)
	if !ok {
		return uuid.Nil, false
	}
	if scopeZoneID != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "solo un administrador puede gestionar cuentas"})
		return uuid.Nil, false
	}
	return userID, true
}

func validRole(role string) bool {
	return role == "administrador" || role == "alimentador"
}

func validAccountState(state string) bool {
	return state == "activada" || state == "pendiente"
}

// resolveZone: el administrador no lleva zona; un alimentador debe tener una.
func resolveZone(c *gin.Context, role string, zoneID *uuid.UUID) (*uuid.UUID, bool) {
	if role == "administrador" {
		// El administrador ve todas las zonas; no se le asigna ninguna.
		return nil, true
	}
	if zoneID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "un alimentador necesita una zona asignada"})
		return nil, false
	}
	return zoneID, true
}

func parseOptionalUUID(c *gin.Context, value *string) (*uuid.UUID, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*value))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zona_id inválido"})
		return nil, false
	}
	return &id, true
}

func ListStaffHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireAdministrador(c, pool); !ok {
			return
		}

		staff, err := repository.ListStaff(pool)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar el personal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"staff": staff})
	}
}

// CreateStaffHandler da de alta una cuenta de staff sin tocar la sesión del administrador
// (a diferencia de POST /auth/admin, que inicia sesión como la cuenta nueva).
func CreateStaffHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireAdministrador(c, pool); !ok {
			return
		}

		var req CreateStaffRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datos inválidos: " + err.Error()})
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "el nombre es obligatorio"})
			return
		}
		if strings.TrimSpace(req.Email) == "" && strings.TrimSpace(req.TelephoneNumber) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere correo o teléfono"})
			return
		}
		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "la contraseña debe tener al menos 6 caracteres"})
			return
		}
		if !validRole(req.Role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rol inválido"})
			return
		}

		zoneID, ok := parseOptionalUUID(c, req.ZoneID)
		if !ok {
			return
		}
		zoneID, ok = resolveZone(c, req.Role, zoneID)
		if !ok {
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo procesar la contraseña"})
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo generar el id"})
			return
		}

		accountState := "activada"
		if req.Activate != nil && !*req.Activate {
			accountState = "pendiente"
		}

		member, err := repository.CreateStaffMember(pool, repository.NewStaffMember{
			ID:              id,
			Name:            req.Name,
			Email:           optionalString(req.Email),
			TelephoneNumber: optionalString(req.TelephoneNumber),
			HashedPassword:  string(hashed),
			Role:            req.Role,
			ZoneID:          zoneID,
			AccountState:    accountState,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				c.JSON(http.StatusConflict, gin.H{"error": "ya existe una cuenta con ese correo o teléfono"})
				return
			}
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "la zona no existe"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo crear la cuenta"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"staff": member})
	}
}

// UpdateStaffHandler cambia rol, zona o estado de la cuenta (activar / suspender).
func UpdateStaffHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID, ok := requireAdministrador(c, pool)
		if !ok {
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
			return
		}

		var req UpdateStaffRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "datos inválidos"})
			return
		}

		current, err := repository.GetStaffMember(pool, id)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la cuenta"})
			return
		}

		role, zoneID, accountState := current.Role, current.ZoneID, current.AccountState
		if req.Role != nil {
			role = *req.Role
		}
		if req.ZoneID != nil {
			if zoneID, ok = parseOptionalUUID(c, req.ZoneID); !ok {
				return
			}
		}
		if req.AccountState != nil {
			accountState = *req.AccountState
		}

		if !validRole(role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rol inválido"})
			return
		}
		if !validAccountState(accountState) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "estado de cuenta inválido"})
			return
		}
		// Evita que el administrador se quite el acceso a sí mismo.
		if id == currentUserID && (role != current.Role || accountState != "activada") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no puedes cambiar tu propio rol ni suspender tu cuenta"})
			return
		}
		if zoneID, ok = resolveZone(c, role, zoneID); !ok {
			return
		}

		member, err := repository.UpdateStaffMember(pool, id, role, zoneID, accountState)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "la zona no existe"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo actualizar la cuenta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"staff": member})
	}
}
