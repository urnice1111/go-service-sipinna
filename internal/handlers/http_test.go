package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"go-service-sipinna/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestLogout(t *testing.T) {
	router := gin.New()
	router.POST("/auth/logout", LogoutHandler)

	w := serve(router, http.MethodPost, "/auth/logout", "")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"Logged out successfully"}`, w.Body.String())
}

func TestAuthValidation(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret"}
	router := gin.New()
	router.POST("/auth/login", LoginHandler(nil, cfg))
	router.POST("/auth/citizen", CitizenSignInHandler(nil, cfg))

	tests := []struct {
		name     string
		path     string
		body     string
		wantCode int
		wantBody string
	}{
		{
			name:     "login invalid json",
			path:     "/auth/login",
			body:     "{",
			wantCode: http.StatusBadRequest,
			wantBody: `{"error":"invalid body"}`,
		},
		{
			name:     "login both email and phone",
			path:     "/auth/login",
			body:     `{"email":"a@b.com","number":"5512345678","password":"secret"}`,
			wantCode: http.StatusBadRequest,
			wantBody: `{"error":"provide either email or number, not both/neither"}`,
		},
		{
			name:     "citizen missing fields",
			path:     "/auth/citizen",
			body:     `{}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "citizen short password",
			path:     "/auth/citizen",
			body:     `{"nombre":"Ana","email":"ana@example.com","password":"corta"}`,
			wantCode: http.StatusBadRequest,
			wantBody: `{"error":"La contraseña debe tener al menos 10 caracteres."}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(router, http.MethodPost, tt.path, tt.body)

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

func serve(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}
