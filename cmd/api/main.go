package main

import (
	"fmt"
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
	router.POST("/auth/logout", handlers.LogoutHandler)
	router.GET("/auth/me", middleware.AuthMiddleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":   c.GetString("user_id"),
			"user_type": c.GetString("user_type"),
		})
	})
	router.PATCH("/admin/:id/status-accepted", middleware.AuthMiddleware(cfg), handlers.AcceptOrRejectAdmin(pool))
	router.POST("/upload", handlers.UploadToS3(uploader))

	report := router.Group("/report")
	report.Use(middleware.AuthRequired())
	{
		report.POST("", handlers.CreateReportHandler(pool))
		report.GET("", handlers.GetUsersReports(pool))
		report.GET("/:zone_id", handlers.GetReportsByZone(pool))
		report.PATCH("/:folio/status", handlers.UpdateReportStatusHandler(pool))
	}

	router.Run(":" + cfg.Port)

}
