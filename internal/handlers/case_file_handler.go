package handlers

import (
	"errors"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UpdateCaseFileHandler modifica el expediente; solo personal activo de la zona.
func UpdateCaseFileHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		folio := strings.TrimSpace(c.Param("report_ref"))

		var req models.CaseFile
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Priority != nil && *req.Priority != "normal" && *req.Priority != "alta" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "priority must be normal or alta"})
			return
		}
		for _, date := range []*string{req.FollowUpDate, req.ClosedDate} {
			if date == nil || *date == "" {
				continue
			}
			if _, err := time.Parse("2006-01-02", *date); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "dates must be YYYY-MM-DD"})
				return
			}
		}
		if req.ChildrenCared != nil && *req.ChildrenCared < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "children_cared must be positive"})
			return
		}

		err := repository.UpdateCaseFile(pool, folio, req, scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if errors.Is(err, repository.ErrUnknownDuplicate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update case file"})
			return
		}

		c.JSON(http.StatusOK, true)
	}
}

// GetReportActionsHandler lista acciones; solo personal activo de la zona.
func GetReportActionsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		actions, err := repository.GetReportActions(pool, strings.TrimSpace(c.Param("report_ref")), scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get actions"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"actions": actions})
	}
}

// AddReportActionRequest es el cuerpo de POST /report/:folio/acciones.
type AddReportActionRequest struct {
	Text string `json:"text" binding:"required,max=2000"`
}

// AddReportActionHandler registra una acción; solo personal activo de la zona.
func AddReportActionHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		var req AddReportActionRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
			return
		}

		action, err := repository.AddReportAction(pool, strings.TrimSpace(c.Param("report_ref")), strings.TrimSpace(req.Text), staffID, scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add action"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"action": action})
	}
}

// GetPublicReportStatusHandler consulta el estado público con folio y clave.
// Responde 404 si no coinciden, sin indicar cuál falló.
func GetPublicReportStatusHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		folio := strings.ToUpper(strings.TrimSpace(c.Param("folio")))
		key, err := uuid.Parse(strings.TrimSpace(c.Query("clave")))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}

		status, err := repository.GetPublicReportStatus(pool, folio, key)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get report"})
			return
		}

		status.Status = repository.PublicStatus(status.Status)
		c.JSON(http.StatusOK, gin.H{"report": status})
	}
}

// GetLogHandler devuelve los últimos 500 accesos; solo administradores.
func GetLogHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireAdministrador(c, pool); !ok {
			return
		}

		entries, err := repository.GetLogEntries(pool, 500)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get log"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"entries": entries})
	}
}
