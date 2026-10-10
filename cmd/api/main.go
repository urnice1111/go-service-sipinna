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
//	POST   /auth/admin                     registro de personal
//	POST   /auth/login                     inicio de sesión con contraseña
//	POST   /auth/google                    inicio de sesión con Google (Supabase)
//	POST   /auth/logout                    cierre de sesión
//	GET    /auth/me                        datos de la sesión actual
//	PATCH  /admin/:id/status-accepted      activa una cuenta de personal pendiente
//	GET    /zones                          zonas visibles para el personal
//	GET    /admin/staff                    lista del personal
//	POST   /admin/staff                    alta de personal
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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
			"Origin", "Content-Type", "Authorization",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.POST("/auth/citizen", handlers.CitizenSignInHandler(pool, cfg))
	router.POST("/auth/admin", handlers.AdminSignInHandler(pool, cfg))
	router.POST("/auth/login", handlers.LoginHandler(pool, cfg))
	router.POST("/auth/google", handlers.GoogleLoginHandler(pool, cfg))
	router.POST("/auth/logout", handlers.LogoutHandler)
	router.GET("/auth/me", middleware.AuthMiddleware(cfg), func(c *gin.Context) {
		// Tokens emitidos antes de agregar los claims "name"/"zone_name" no los traen; se regresan vacíos.
		name, zoneName := "", ""
		if claims, ok := c.Get("user"); ok {
			if mapClaims, ok := claims.(jwt.MapClaims); ok {
				name, _ = mapClaims["name"].(string)
				zoneName, _ = mapClaims["zone_name"].(string)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"user_id":   c.GetString("user_id"),
			"user_type": c.GetString("user_type"),
			"name":      name,
			"zone_name": zoneName,
		})
	})
	router.PATCH("/admin/:id/status-accepted", middleware.AuthMiddleware(cfg), handlers.AcceptOrRejectAdmin(pool))
	router.GET("/zones", middleware.AuthRequired(), handlers.GetZones(pool))

	// Gestión de cuentas solo para administradores activos.
	staff := router.Group("/admin/staff")
	staff.Use(middleware.AuthMiddleware(cfg))
	{
		staff.GET("", handlers.ListStaffHandler(pool))
		staff.POST("", handlers.CreateStaffHandler(pool))
		staff.PATCH("/:id", handlers.UpdateStaffHandler(pool))
	}

	report := router.Group("/report")
	report.Use(middleware.AuthRequired())
	{
		report.POST("", handlers.CreateReportHandler(pool, cfg))
		report.GET("", handlers.GetUsersReports(pool))
		report.GET("/all", handlers.GetAllReports(pool))
		report.GET("/zone/:zone_id", handlers.GetReportsByZone(pool))
		report.GET("/:folio", handlers.GetReportByFolioHandler(pool))
		report.PATCH("/:folio/status", handlers.UpdateReportStatusHandler(pool))
		report.DELETE("/:folio", handlers.DeleteReportHandler(pool, uploader))

		report.POST("/:report_id/images", handlers.RegisterImagesRows(pool))
		report.PUT("/:report_id/images/:image_id", handlers.UploadToS3(uploader, pool))
		report.PUT("/:report_id/submit", handlers.UpdateReport(pool))
	}

	router.Run(":" + cfg.Port)

}
