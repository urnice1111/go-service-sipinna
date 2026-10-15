package handlers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/repository"
	"go-service-sipinna/internal/resend"
	"go-service-sipinna/internal/twilio_messages"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twilio/twilio-go"
	"golang.org/x/crypto/bcrypt"
)

const (
	// passwordResetCodeTTL es la vigencia de un código de recuperación.
	passwordResetCodeTTL = 10 * time.Minute
	// passwordResetCooldown es el tiempo mínimo entre dos códigos para el mismo usuario.
	passwordResetCooldown = time.Minute
)

// forgotPasswordMessage se responde exista o no la cuenta, para no revelar qué
// correos están registrados.
const forgotPasswordMessage = "Si el correo está registrado, te enviamos un código de 6 dígitos. Vence en 10 minutos."

// ForgotPasswordRequest es el cuerpo de POST /auth/password/forgot. Debe traer
// exactamente uno de Email o Phone.
type ForgotPasswordRequest struct {
	Email *string `json:"email"`
	Phone *string `json:"number"`
}

// ResetPasswordRequest es el cuerpo de POST /auth/password/reset.
type ResetPasswordRequest struct {
	Email       *string `json:"email"`
	Phone       *string `json:"number"`
	Code        string  `json:"code" binding:"required"`
	NewPassword string  `json:"new_password" binding:"required"`
}

// ForgotPasswordHandler maneja POST /auth/password/forgot: genera un código de 6
// dígitos y lo envía por correo con Resend. Por ahora solo se admite correo; con
// teléfono responde 400.
//
// Responde 200 con {"message": ...} aunque el correo no esté registrado, 429 si se
// pidió otro código hace menos de un minuto y 502 si Resend no pudo enviar el correo.
func ForgotPasswordHandler(pool *pgxpool.Pool, cfg *config.Config, client *twilio.RestClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ForgotPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		email, phone, ok := passwordResetEmail(c, req.Email, req.Phone)
		if !ok {
			return
		}

		user, err := repository.GetUserByContact(pool, email, phone)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"message": forgotPasswordMessage})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la solicitud."})
			return
		}

		// Una cuenta invitada define su contraseña al aceptar la invitación, no aquí. Se
		// responde igual que si no existiera para no revelar que hay una invitación.
		invited, err := repository.IsInvitationPending(pool, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la solicitud."})
			return
		}
		if invited {
			c.JSON(http.StatusOK, gin.H{"message": forgotPasswordMessage})
			return
		}

		code, err := generateResetCode()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el código."})
			return
		}

		codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el código."})
			return
		}

		err = repository.CreatePasswordResetCode(pool, user.ID, string(codeHash), passwordResetCodeTTL, passwordResetCooldown)
		if errors.Is(err, repository.ErrResetCodeTooSoon) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Ya te enviamos un código. Espera un minuto antes de pedir otro."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la solicitud."})
			return
		}

		minutes := int(passwordResetCodeTTL / time.Minute)

		if email != "" {
			if err := resend.SendPasswordResetCode(c.Request.Context(), cfg, email, user.Name, code, minutes); err != nil {
				log.Printf("could not send password reset email to user %s: %v", user.ID, err)
				if err := repository.DiscardPasswordResetCode(pool, user.ID); err != nil {
					log.Printf("could not discard password reset code of user %s: %v", user.ID, err)
				}
				c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudo enviar el correo. Inténtalo de nuevo más tarde."})
				return
			}
		}
		if phone != "" {
			body := "Tu codigo para recuperar tu cuenta es: " + code
			if err := twilio_messages.SendSMS(client, body, phone); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": forgotPasswordMessage})
	}
}

// ResetPasswordHandler maneja POST /auth/password/reset: si el código de
// recuperación es correcto cambia la contraseña (mínimo 6 caracteres). No inicia
// sesión; el usuario debe entrar con la nueva contraseña.
//
// Responde 400 si el código no es válido, ya venció o se bloqueó tras
// [repository.MaxPasswordResetAttempts] intentos fallidos.
func ResetPasswordHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ResetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		email, phone, ok := passwordResetEmail(c, req.Email, req.Phone)
		if !ok {
			return
		}

		if len(req.NewPassword) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La contraseña debe tener al menos 6 caracteres."})
			return
		}

		invalidCode := gin.H{"error": "El código no es válido o ya venció."}

		user, err := repository.GetUserByContact(pool, email, phone)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, invalidCode)
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la solicitud."})
			return
		}

		invited, err := repository.IsInvitationPending(pool, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la solicitud."})
			return
		}
		if invited {
			c.JSON(http.StatusBadRequest, invalidCode)
			return
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cambiar la contraseña."})
			return
		}

		err = repository.ResetPasswordWithCode(pool, user.ID, req.Code, string(newHash))
		if errors.Is(err, repository.ErrInvalidResetCode) {
			c.JSON(http.StatusBadRequest, invalidCode)
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cambiar la contraseña."})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Tu contraseña se actualizó. Ya puedes iniciar sesión."})
	}
}

// passwordResetEmail valida que el cuerpo traiga solo correo y lo regresa recortado.
// Si regresa false, la respuesta de error ya se escribió.
func passwordResetEmail(c *gin.Context, email, phone *string) (string, string, bool) {

	gottenEmail := stringValue(email)
	gottenPhone := stringValue(phone)

	if gottenEmail == "" && gottenPhone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nor email or phone provided"})
		return "", "", false
	}

	if gottenEmail != "" && gottenPhone != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provided both, expected only one"})
		return "", "", false
	}

	if gottenEmail != "" {
		return gottenEmail, "", true
	}

	return "", gottenPhone, true
}

// generateResetCode regresa un código aleatorio de 6 dígitos.
func generateResetCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
