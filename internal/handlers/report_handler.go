package handlers

import (
	"fmt"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateReport struct {
	Description      string   `json:"description" db:"descripcion"`
	Latitude         float32  `json:"latitude" db:"latitud"`
	Longitude        float32  `json:"longitude" db:"longitud"`
	ChildrenQuantity int      `json:"children_quantity" db:"cantidad_ninos"`
	ChildrenAge      string   `json:"children_age" db:"edad_ninos"`
	WorkType         string   `json:"work_type" db:"tipo_trabajo"`
	SightingTime     string   `json:"sighting_time" db:"horario_avistamiento"`
	Photos           []string `json:"photos,omitempty"`
}

func CreateReportHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		nonExisting := c.GetBool("is_admin")
		if !nonExisting {
			fmt.Println("no existe")
		} else {
			fmt.Println("si existe")
		}

		fmt.Println(userID)
		var req CreateReport
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		if req.Photos == nil {
			req.Photos = []string{}
		}

		idV7, err := uuid.NewV7()

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate uuid" + err.Error()})
			return
		}

		report := &models.Report{
			ID:               idV7,
			Description:      req.Description,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			ChildrenQuantity: req.ChildrenQuantity,
			ChildrenAge:      req.ChildrenAge,
			WorkType:         req.WorkType,
			SightingTime:     req.SightingTime,
		}

		newReport, err := repository.CreateReport(pool, report, req.Photos, &userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reporte": newReport})

	}

}

func GetReportsByZone(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var zoneID string = c.Param("zone_id")

		if zoneID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you must provide a zone id"})
			return
		}

		var err error
		var reports []models.IndividualReport

		reports, err = repository.GetReportsByZone(pool, zoneID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})

	}
}
