package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GoogleLoginRequest es el cuerpo de POST /auth/google.
type GoogleLoginRequest struct {
	// access_token de la sesión de Supabase que obtuvo el frontend tras el login con Google
	AccessToken string `json:"access_token" binding:"required"`
}

// supabaseUser son los campos que se usan de GET {SUPABASE_URL}/auth/v1/user.
type supabaseUser struct {
	Email            string `json:"email"`
	EmailConfirmedAt string `json:"email_confirmed_at"`
	UserMetadata     struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
	} `json:"user_metadata"`
}

// errInvalidSupabaseToken indica que Supabase rechazó el token (401/403).
var errInvalidSupabaseToken = errors.New("invalid supabase token")

// supabaseHTTPClient es el cliente HTTP para llamar a Supabase Auth.
var supabaseHTTPClient = &http.Client{Timeout: 5 * time.Second}

// GoogleLoginHandler maneja POST /auth/google: cambia la sesión de Supabase (Google)
// por la cookie de sesión propia.
//
// Si no existe una cuenta con el correo de Google, crea un ciudadano nuevo. Responde
// 401 si el token no es válido o el correo no está verificado, 502 si no se pudo
// contactar a Supabase y 500 si el login con Google no está configurado.
func GoogleLoginHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.SupabaseURL == "" || cfg.SupabaseAnonKey == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Google login is not configured"})
			return
		}

		var req GoogleLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}

		googleUser, err := fetchSupabaseUser(c.Request.Context(), cfg, req.AccessToken)
		if errors.Is(err, errInvalidSupabaseToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Google session"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Could not verify Google session"})
			return
		}

		email := strings.ToLower(strings.TrimSpace(googleUser.Email))
		if email == "" || googleUser.EmailConfirmedAt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Google account email is not verified"})
			return
		}

		user, err := repository.GetUserByContact(pool, email, "")
		if errors.Is(err, pgx.ErrNoRows) {
			user, err = createGoogleCitizen(pool, email, googleUser)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
			return
		}

		userType, isAdmin, zoneName, err := repository.GetUserType(pool, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user type"})
			return
		}

		respondWithToken(c, http.StatusOK, user.ID, cfg, isAdmin, userType, user.Name, zoneName)
	}
}

// fetchSupabaseUser valida el token directamente con Supabase Auth, que también revisa que no esté revocado.
func fetchSupabaseUser(ctx context.Context, cfg *config.Config, accessToken string) (*supabaseUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.SupabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	resp, err := supabaseHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errInvalidSupabaseToken
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("supabase auth returned %d", resp.StatusCode)
	}

	var user supabaseUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// createGoogleCitizen registra un ciudadano con el correo y el nombre de la cuenta de
// Google; si no hay nombre, usa la parte local del correo.
//
// Las cuentas creadas con Google no tienen contraseña: password_hash queda vacío y el login
// con contraseña siempre falla hasta que el usuario defina una con "Olvidé mi contraseña".
func createGoogleCitizen(pool *pgxpool.Pool, email string, googleUser *supabaseUser) (*models.User, error) {
	name := strings.TrimSpace(googleUser.UserMetadata.FullName)
	if name == "" {
		name = strings.TrimSpace(googleUser.UserMetadata.Name)
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	idV7, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	return repository.CreateUser(pool, &models.User{
		ID:    idV7,
		Name:  name,
		Email: &email,
	})
}
