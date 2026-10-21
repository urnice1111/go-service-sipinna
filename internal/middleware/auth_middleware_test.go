package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go-service-sipinna/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

const testSecret = "test-secret"

func TestAuthMiddleware(t *testing.T) {
	valid := signToken(t, testSecret)

	tests := []struct {
		name     string
		secret   string
		cookie   string
		wantCode int
		wantBody string
	}{
		{"no cookie", testSecret, "", http.StatusUnauthorized, ""},
		{"garbage token", testSecret, "not-a-jwt", http.StatusUnauthorized, ""},
		{"empty jwt secret", "", "any-token", http.StatusInternalServerError, ""},
		{"valid session", testSecret, valid, http.StatusOK, "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(AuthMiddleware(&config.Config{JWTSecret: tt.secret}))
			router.GET("/protected", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "session_token", Value: tt.cookie})
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestOptionalAuthWithoutCookie(t *testing.T) {
	router := gin.New()
	router.Use(OptionalAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func signToken(t *testing.T, secret string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id":   "11111111-1111-1111-1111-111111111111",
		"is_admin":  false,
		"user_type": "citizen",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}
