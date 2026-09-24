package handlers

import (
	"errors"
	"fmt"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		if zoneID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you must provide a zone id"})
			return
		}

		var err error
		var reports []models.IndividualReport

		reports, err = repository.GetReportsByZone(pool, zoneID, scopeZoneID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})

	}
}

func GetUsersReports(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")

		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No user id"})
			return
		}

		reportsBrief, err := repository.GetReportsSummaryOfUser(pool, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reportsBrief})

	}
}

type UpdateReportStatusRequest struct {
	Status string `json:"estado" binding:"required"`
	Reason string `json:"motivo"`
}

var validReportStatuses = map[string]bool{
	"registrado":     true,
	"en_revision":    true,
	"en_seguimiento": true,
	"canalizado":     true,
	"concluido":      true,
	"archivado":      true,
	"cancelado":      true,
	"reincidente":    true,
}

var reportStatusesRequiringReason = map[string]bool{
	"cancelado":   true,
	"archivado":   true,
	"reincidente": true,
}

func UpdateReportStatusHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		folio := strings.TrimSpace(c.Param("folio"))

		var req UpdateReportStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		req.Status = strings.TrimSpace(req.Status)
		req.Reason = strings.TrimSpace(req.Reason)

		if !validReportStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid estado"})
			return
		}

		if reportStatusesRequiringReason[req.Status] && req.Reason == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "motivo is required for estado " + req.Status})
			return
		}

		changedAt, err := repository.UpdateReportStatus(pool, folio, req.Status, req.Reason, staffID, scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if errors.Is(err, repository.ErrSameReportStatus) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update report status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"folio":            folio,
			"estado":           req.Status,
			"state_changed_at": changedAt,
		})
	}
}

// requireActiveStaff checks against the db that the user is an activated 'administrador' or
// 'alimentador'. It returns the user id and the zone the user is restricted to (nil for
// 'administrador'). When it returns false the error response has already been written.
func requireActiveStaff(c *gin.Context, pool *pgxpool.Pool) (uuid.UUID, *uuid.UUID, bool) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return uuid.Nil, nil, false
	}

	staff, err := repository.GetActiveStaff(pool, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusForbidden, gin.H{"error": "an activated administrador or alimentador account is required"})
		return uuid.Nil, nil, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not verify user permissions"})
		return uuid.Nil, nil, false
	}

	if staff.Role == "administrador" {
		return userID, nil, true
	}

	if staff.ZoneID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "alimentador has no zone assigned"})
		return uuid.Nil, nil, false
	}

	return userID, staff.ZoneID, true
}
