package main

import (
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/database"
	"go-service-sipinna/internal/handlers"
	"log"

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

	var pool *pgxpool.Pool
	pool, err = database.Connect(cfg.DatabaseURL)

	if err != nil {
		log.Fatal("Failed to connect to database", err)
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

	router.POST("/user", handlers.CitizenSignInHandler(pool))

	router.Run(":" + cfg.Port)

}
