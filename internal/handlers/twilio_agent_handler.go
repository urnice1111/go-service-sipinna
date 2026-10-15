package handlers

import (
	"fmt"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateReportFromTwilio(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		twilio_key := c.GetHeader("twilio")

		fmt.Println(twilio_key)
		fmt.Println(cfg.TwilioKey)

		if (twilio_key == "") || (twilio_key != cfg.TwilioKey) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bad secret key"})
			return
		}

		var req models.ElevenLabsRequest

		err := c.ShouldBindJSON(&req)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Bad body"})
			return
		}

		coordinates, err := GetCoordinates(cfg)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		report, err := repository.InsertReportElevenLabs(pool, coordinates, &req)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		c.JSON(http.StatusOK, gin.H{"report_id": report.ReportID})

	}
}

// Helpers
func GetCoordinates(cfg *config.Config) (*models.GeoCode, error) {
	// TODO:
	temp := models.GeoCode{Latitude: 19.595163, Longitude: -99.227264}
	return &temp, nil
}
