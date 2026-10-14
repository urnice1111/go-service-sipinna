// Command api es el servidor HTTP del backend de SIPINNA, el sistema para reportar
// posibles casos de trabajo infantil.
//
// Al arrancar carga la configuración, se conecta a PostgreSQL, prepara el cliente
// de S3, lanza en segundo plano el análisis de sospecha de reportes y expone la API
// REST con Gin en el puerto definido por PORT.
//
// Rutas principales:
//
//	POST   /auth/citizen                   registro de ciudadano
//	POST   /auth/login                     inicio de sesión con contraseña
//	POST   /auth/google                    inicio de sesión con Google (Supabase)
//	POST   /auth/logout                    cierre de sesión
//	POST   /auth/password/forgot           envía por correo (Resend) un código de recuperación
//	POST   /auth/password/reset            cambia la contraseña con ese código
//	POST   /auth/invitation                datos de una invitación de personal
//	POST   /auth/invitation/accept         acepta la invitación, define la contraseña e inicia sesión
//	POST   /auth/invitation/reject         rechaza la invitación y borra la cuenta
//	GET    /auth/me                        datos de la sesión actual
//	GET    /account                       perfil de la cuenta actual
//	PUT    /account                       edición del perfil de ciudadano
//	DELETE /account                       elimina la cuenta actual (cualquier rol)
//	GET    /zones                          zonas visibles para el personal
//	GET    /admin/staff                    lista del personal
//	POST   /admin/staff                    alta de personal (envía la invitación por correo)
//	POST   /admin/staff/:id/invitation     reenvía la invitación
//	PATCH  /admin/staff/:id                cambio de rol, zona o estado de cuenta
//	POST   /report                         crea un reporte (borrador)
//	GET    /report                         reportes del ciudadano
//	GET    /report/all                     todos los reportes visibles para el personal
//	GET    /report/zone/:zone_id           reportes de una zona
//	GET    /report/:folio                  detalle de un reporte
//	PATCH  /report/:folio/status           cambia el estado de un reporte
//	DELETE /report/:folio                  elimina un reporte y sus fotos
//	POST   /report/:report_id/images       registra las fotos que se van a subir
//	PUT    /report/:report_id/images/:id   sube una foto a S3
//	PUT    /report/:report_id/submit       envía el reporte (deja de ser borrador)
package main

import (
	"context"
	"fmt"
	"go-service-sipinna/internal/analysis"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/database"
	"go-service-sipinna/internal/handlers"
	"go-service-sipinna/internal/middleware"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var cfg *config.Config
	var err error
	cfg, err = config.Load()

	if err != nil {
		log.Fatal("Failed to load configuration", err)
	}

	if err := os.MkdirAll("./uploads", 0755); err != nil {
		panic(fmt.Sprintf("Failed to create uploads directory: %v", err))
	}

	var pool *pgxpool.Pool
	pool, err = database.Connect(cfg.DatabaseURL)

	if err != nil {
		log.Fatal("Failed to connect to database", err)
	}

	/*Handler para nuevo s3 uploader que esta definido en handlers*/

	uploader, err := handlers.NewS3Uploader(cfg.SipinnaBucket)

	if err != nil {
		panic(err)
	}

	defer pool.Close()

	// Analiza en segundo plano qué tan probable es que cada reporte nuevo sea falso
	// y guarda el resultado en la columna "sospechoso" (0 = legítimo, 1 = falso).
	go analysis.NewWorker(pool, analysis.NewJevClient(cfg.TypeSafeAPIKey)).Run(context.Background())

	var router *gin.Engine = gin.Default()
	router.SetTrustedProxies(nil)
	router.GET("/", func(c *gin.Context) {
		//Returns a map: [string]any{}
		c.JSON(200, gin.H{
			"message":  "Todo API is running well",
			"status":   "success",
			"database": "connected",
		})
	})

	allowedOrigins := []string{
		"http://localhost:5173", // Vite
		"http://localhost:3000", // Create React App
		"https://www.sipinna.com",
	}
	if frontendURL := strings.TrimSpace(cfg.FrontendURL); frontendURL != "" {
		allowedOrigins = append(allowedOrigins, strings.TrimRight(frontendURL, "/"))
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Authorization", "X-Edit-Token",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(middleware.AccountSession(pool, cfg))

	router.POST("/auth/citizen", handlers.CitizenSignInHandler(pool, cfg))
	router.POST("/auth/login", handlers.LoginHandler(pool, cfg))
	router.POST("/auth/google", handlers.GoogleLoginHandler(pool, cfg))
	router.POST("/auth/logout", handlers.LogoutHandler)
	router.POST("/auth/password/forgot", handlers.ForgotPasswordHandler(pool, cfg))
	router.POST("/auth/password/reset", handlers.ResetPasswordHandler(pool))
	router.POST("/auth/invitation", handlers.GetInvitationHandler(pool))
	router.POST("/auth/invitation/accept", handlers.AcceptInvitationHandler(pool, cfg))
	router.POST("/auth/invitation/reject", handlers.RejectInvitationHandler(pool))
	router.GET("/auth/me", middleware.AuthMiddleware(cfg), handlers.MeHandler(pool))
	account := router.Group("/account", middleware.AuthMiddleware(cfg))
	account.GET("", handlers.GetProfileHandler(pool))
	account.PUT("", handlers.UpdateProfileHandler(pool))
	account.DELETE("", handlers.DeleteAccountHandler(pool))
	router.GET("/zones", middleware.AuthRequired(), handlers.GetZones(pool))

	// Gestión de cuentas solo para administradores activos.
	staff := router.Group("/admin/staff")
	staff.Use(middleware.AuthMiddleware(cfg))
	{
		staff.GET("", handlers.ListStaffHandler(pool))
		staff.POST("", handlers.CreateStaffHandler(pool, cfg))
		staff.POST("/:id/invitation", handlers.ResendInvitationHandler(pool, cfg))
		staff.PATCH("/:id", handlers.UpdateStaffHandler(pool))
	}

	report := router.Group("/report")
	report.POST("", middleware.OptionalAuth(), handlers.CreateReportHandler(pool, cfg))
	report.POST("/:report_id/images", handlers.RegisterImagesRows(pool))
	report.PUT("/:report_id/images/:image_id", handlers.UploadToS3(uploader, pool))
	report.PUT("/:report_id/submit", handlers.UpdateReport(pool, cfg))

	authedReport := report.Group("")
	authedReport.Use(middleware.AuthRequired())
	{
		authedReport.GET("", handlers.GetUsersReports(pool))
		authedReport.GET("/all", handlers.GetAllReports(pool))
		authedReport.GET("/zone/:zone_id", handlers.GetReportsByZone(pool))
		authedReport.GET("/:folio", handlers.GetReportByFolioHandler(pool))
		authedReport.PATCH("/:folio/status", handlers.UpdateReportStatusHandler(pool))
		authedReport.PATCH("/:folio/assignment", handlers.ReportAssignment(pool))
		authedReport.DELETE("/:folio", handlers.DeleteReportHandler(pool, uploader))
	}

	router.Run(":" + cfg.Port)

}
