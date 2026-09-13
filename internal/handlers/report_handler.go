package handlers

import (
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Lo que manda la app. Los nombres coinciden con ReporteRequest en Android.
type CreateReportRequest struct {
	Description   string  `json:"descripcion" binding:"required"`
	Latitude      float64 `json:"latitud"`
	Longitude     float64 `json:"longitud"`
	Address       string  `json:"direccion"`
	ChildrenCount int     `json:"cantidad_ninos"`
	ChildrenAge   string  `json:"edad_ninos"`
	WorkType      string  `json:"tipo_trabajo"`
	SightingTime  string  `json:"horario_avistamiento"`
	Condition     string  `json:"condicion"`
}

// POST /reporte (requiere token)
func CreateReportHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateReportRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// RF06: sin descripcion o sin ubicacion no se acepta el reporte
		if strings.TrimSpace(req.Description) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "la descripcion es obligatoria"})
			return
		}

		if req.Latitude == 0 && req.Longitude == 0 && strings.TrimSpace(req.Address) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "la ubicacion es obligatoria"})
			return
		}

		// El id del usuario lo dejo el middleware al validar el token
		var userID *uuid.UUID
		if valor, existe := c.Get("userID"); existe {
			if parsed, err := uuid.Parse(valor.(string)); err == nil {
				userID = &parsed
			}
		}

		idV7, err := uuid.NewV7()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate uuid" + err.Error()})
			return
		}

		var cantidad int = req.ChildrenCount
		if cantidad < 1 {
			cantidad = 1
		}

		report := &models.Report{
			ID:            idV7,
			Folio:         generateFolio(idV7),
			UserID:        userID,
			Description:   strings.TrimSpace(req.Description),
			Latitude:      req.Latitude,
			Longitude:     req.Longitude,
			Address:       req.Address,
			ChildrenCount: cantidad,
			ChildrenAge:   req.ChildrenAge,
			WorkType:      req.WorkType,
			SightingTime:  req.SightingTime,
			Condition:     req.Condition,
			State:         "registrado", // primer estado segun RF09
		}

		newReport, err := repository.CreateReport(pool, report)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving the report on the db" + err.Error()})
			return
		}

		// RF07: se responde con el folio para que el ciudadano lo guarde
		c.JSON(http.StatusCreated, gin.H{
			"id":     newReport.ID,
			"folio":  newReport.Folio,
			"estado": newReport.State,
		})
	}
}

// Folio legible para el ciudadano, ej: SIP-20260913-A3F2B1
// Usa la fecha + los ultimos 6 caracteres del uuid (parte aleatoria)
func generateFolio(id uuid.UUID) string {
	var fecha string = time.Now().Format("20060102")
	var hex string = strings.ReplaceAll(id.String(), "-", "")
	var sufijo string = strings.ToUpper(hex[len(hex)-6:])

	return "SIP-" + fecha + "-" + sufijo
}
