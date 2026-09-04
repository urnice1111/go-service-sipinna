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

	router.POST("/auth/citizen", handlers.CitizenSignInHandler(pool, cfg))
	router.POST("/auth/admin", handlers.AdminSignInHandler(pool, cfg))
	router.POST("/auth/login", handlers.LoginHandler(pool, cfg))
	router.GET("/auth/me", middleware.AuthMiddleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	router.POST("/upload", handlers.UploadToS3(uploader))
	router.POST("/report", middleware.AuthMiddleware(cfg), handlers.CreateReportHandler(pool))

	router.Run(":" + cfg.Port)

}
