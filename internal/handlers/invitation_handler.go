package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/repository"
	"go-service-sipinna/internal/resend"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// invitationTTL es cuánto tiempo es válido el enlace de una invitación.
const invitationTTL = 48 * time.Hour

// invitationPath es la página del frontend que recibe el token de la invitación.
const invitationPath = "/aceptar-invitacion"

// roleLabels es el nombre de cada rol tal como se muestra en el correo.
var roleLabels = map[string]string{
	"administrador": "Administrador",
	"alimentador":   "Alimentador",
}

// InvitationTokenRequest es el cuerpo de POST /auth/invitation y POST
// /auth/invitation/reject. El token va en el cuerpo y no en la URL para que no quede
// en los logs de acceso.
type InvitationTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// AcceptInvitationRequest es el cuerpo de POST /auth/invitation/accept.
type AcceptInvitationRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// hashInvitationToken regresa el SHA-256 en hex de token, que es lo que se guarda en
// la base. El token tiene 256 bits aleatorios, así que no hace falta bcrypt.
func hashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// invitationsConfigured indica si se pueden armar enlaces de invitación. Si regresa
// false, la respuesta 500 ya se escribió.
func invitationsConfigured(c *gin.Context, cfg *config.Config) bool {
	if strings.TrimSpace(cfg.FrontendURL) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "las invitaciones no están configuradas (FRONTEND_URL)"})
		return false
	}
	return true
}

// sendStaffInvitation genera un token nuevo para member, lo guarda (reemplazando los
// anteriores) y envía el correo con el enlace. Si regresa false, la respuesta de
// error ya se escribió.
func sendStaffInvitation(c *gin.Context, pool *pgxpool.Pool, cfg *config.Config, member *repository.StaffMember) bool {
	if member.Email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "la cuenta no tiene correo"})
		return false
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo generar la invitación"})
		return false
	}
	token := hex.EncodeToString(tokenBytes)

	if err := repository.CreateInvitation(pool, member.ID, hashInvitationToken(token), invitationTTL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo generar la invitación"})
		return false
	}

	link := strings.TrimRight(strings.TrimSpace(cfg.FrontendURL), "/") + invitationPath + "?token=" + url.QueryEscape(token)
	hours := int(invitationTTL / time.Hour)
	if err := resend.SendStaffInvitation(c.Request.Context(), cfg, *member.Email, member.Name, roleLabels[member.Role], link, hours); err != nil {
		log.Printf("could not send invitation email to staff %s: %v", member.ID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "no se pudo enviar el correo de invitación"})
		return false
	}

	return true
}

// ResendInvitationHandler maneja POST /admin/staff/:id/invitation: envía una
// invitación nueva a una cuenta que todavía no la acepta; el enlace anterior deja de
// servir. Solo para administradores.
//
// Responde 409 si la cuenta ya aceptó su invitación (o está suspendida) y 502 si no
// se pudo enviar el correo.
func ResendInvitationHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireAdministrador(c, pool); !ok {
			return
		}
		if !invitationsConfigured(c, cfg) {
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
			return
		}

		member, err := repository.GetStaffMember(pool, id)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cuenta no encontrada"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo cargar la cuenta"})
			return
		}

		// Una cuenta suspendida también es 'pendiente', pero ya tiene contraseña; reenviarle
		// la invitación le permitiría reactivarse sola.
		if member.AccountState != "pendiente" || !member.InvitationPending {
			c.JSON(http.StatusConflict, gin.H{"error": "la cuenta ya aceptó su invitación"})
			return
		}

		if !sendStaffInvitation(c, pool, cfg, member) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "invitación enviada"})
	}
}

// GetInvitationHandler maneja POST /auth/invitation: regresa nombre, correo, rol y
// zona de la cuenta invitada para mostrarlos antes de aceptar. Es POST porque el token
// va en el cuerpo. Responde 404 si la invitación no es válida o ya venció.
func GetInvitationHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req InvitationTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "falta el token"})
			return
		}

		member, err := repository.GetInvitation(pool, hashInvitationToken(req.Token))
		if errors.Is(err, repository.ErrInvalidInvitation) {
			c.JSON(http.StatusNotFound, gin.H{"error": "La invitación no es válida o ya venció."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la invitación."})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"nombre":      member.Name,
			"email":       member.Email,
			"rol":         member.Role,
			"zona_nombre": member.ZoneName,
		})
	}
}

// AcceptInvitationHandler maneja POST /auth/invitation/accept: guarda la contraseña
// (mínimo 6 caracteres), activa la cuenta e inicia su sesión. Responde 400 si la
// invitación no es válida o ya venció.
func AcceptInvitationHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AcceptInvitationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La contraseña debe tener al menos 6 caracteres."})
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo activar la cuenta."})
			return
		}

		member, err := repository.AcceptInvitation(pool, hashInvitationToken(req.Token), string(hashed))
		if errors.Is(err, repository.ErrInvalidInvitation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La invitación no es válida o ya venció."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo activar la cuenta."})
			return
		}

		userType, isAdmin, zoneName, err := repository.GetUserType(pool, member.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user type"})
			return
		}

		respondWithToken(c, http.StatusOK, member.ID, cfg, isAdmin, userType, member.Name, zoneName)
	}
}

// RejectInvitationHandler maneja POST /auth/invitation/reject: borra la cuenta
// invitada. Responde 400 si la invitación no es válida o ya venció.
func RejectInvitationHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req InvitationTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		err := repository.RejectInvitation(pool, hashInvitationToken(req.Token))
		if errors.Is(err, repository.ErrInvalidInvitation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La invitación no es válida o ya venció."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo rechazar la invitación."})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Invitación rechazada."})
	}
}
