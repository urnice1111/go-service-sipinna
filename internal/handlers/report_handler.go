package handlers

import (
	"errors"
	"fmt"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"go-service-sipinna/internal/resend"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateReport es el cuerpo de POST /report.
type CreateReport struct {
	Description      string  `json:"description" db:"descripcion"`
	Latitude         float32 `json:"latitude" db:"latitud"`
	Longitude        float32 `json:"longitude" db:"longitud"`
	ChildrenQuantity int     `json:"children_quantity" db:"cantidad_ninos"`
	ChildrenAge      string  `json:"children_age" db:"edad_ninos"`
	WorkType         string  `json:"work_type" db:"tipo_trabajo"`
	SightingTime     string  `json:"sighting_time" db:"horario_avistamiento"`
}

// CreateReportHandler maneja POST /report: crea un reporte a nombre del usuario de
// la sesión. La zona y el folio (RIETI-<MUNICIPIO>-<AÑO>-<consecutivo>) se asignan
// en la base de datos según la zona más cercana a las coordenadas.
//
// El reporte nace como borrador; se envía con [UpdateReport] cuando terminan de
// subirse sus fotos. Responde 200 con {"reporte_id": <uuid>}.
func CreateReportHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		nonExisting := c.GetBool("is_admin")
		if !nonExisting {
			fmt.Println("no existe")
		} else {
			fmt.Println("si existe")
		}

		resend.SendMessage(cfg, "emilianogarram2910@gmail.com")

		fmt.Println(userID)
		var req CreateReport
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
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

		newReport, err := repository.CreateReport(pool, report, &userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reporte_id": newReport.ID})

	}

}

// RegisterImagesRows maneja POST /report/:report_id/images: registra en
// imagenes_reporte una fila "pendiente" por cada imagen del cuerpo
// ([models.ImagesRequest]) y le asigna una llave única en S3.
//
// Responde 200 con {"success": [<id de imagen>, ...]}; con esos ids el cliente sube
// cada archivo mediante [UploadToS3].
func RegisterImagesRows(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		reportID := c.Param("report_id")

		if reportID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "You must provide a valid report id"})
			return
		}

		fmt.Println(reportID)

		var req models.ImagesRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No images"})
			return
		}

		imagesList := &models.ImagesRequest{
			Images: req.Images,
		}

		imagesIDs, err := repository.WriteImages(pool, reportID, *imagesList)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": imagesIDs})
	}
}

// GetReportsByZone maneja GET /report/zone/:zone_id: regresa los reportes de una
// zona, identificada por su id o por el nombre del municipio. Solo para personal
// activo; a un alimentador además se le limita a su propia zona.
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

// GetAllReports maneja GET /report/all: regresa todos los reportes al 'administrador';
// el 'alimentador' solo recibe los de su zona asignada.
func GetAllReports(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		reports, err := repository.GetReportsByZone(pool, "", scopeZoneID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})
	}
}

// GetUsersReports maneja GET /report: regresa el resumen de los reportes creados por
// el usuario de la sesión ([models.IndividualReportInfoBrief]).
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

// UpdateReportStatusRequest es el cuerpo de PATCH /report/:folio/status.
// Reason (motivo) es obligatorio para los estados de reportStatusesRequiringReason.
type UpdateReportStatusRequest struct {
	Status string `json:"estado" binding:"required"`
	Reason string `json:"motivo"`
}

// validReportStatuses son los estados a los que el personal puede mover un reporte
// (valores del enum report_status, excepto DRAFT).
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

// reportStatusesRequiringReason son los estados que exigen un motivo al asignarse.
var reportStatusesRequiringReason = map[string]bool{
	"cancelado":   true,
	"archivado":   true,
	"reincidente": true,
}

// UpdateReportStatusHandler maneja PATCH /report/:folio/status: agrega un nuevo
// estado al historial del reporte. Solo para personal activo y dentro de su zona.
//
// Responde 400 si el estado no es válido o falta el motivo, 404 si el reporte no
// existe o está fuera de la zona, y 409 si el reporte ya tiene ese estado.
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

// requireActiveStaff verifica en la base de datos que el usuario de la sesión sea un
// 'administrador' o 'alimentador' con cuenta activada. Regresa su id y la zona a la que
// está restringido (nil para 'administrador'). Si regresa false, la respuesta de error
// ya se escribió y el handler solo debe terminar.
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

// GetReportByFolioHandler regresa el detalle de un reporte.
// - Personal (administrador / alimentador activado): cualquier reporte de su zona.
// - Ciudadano: solo sus propios reportes, sin el nivel de sospecha ni su nombre.
func GetReportByFolioHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		folio := strings.TrimSpace(c.Param("folio"))
		if folio == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you must provide a folio"})
			return
		}

		var scopeZoneID, citizenID *uuid.UUID
		if c.GetString("user_type") == "citizen" {
			id, err := uuid.Parse(c.GetString("user_id"))
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
				return
			}
			citizenID = &id
		} else {
			_, zoneID, ok := requireActiveStaff(c, pool)
			if !ok {
				return
			}
			scopeZoneID = zoneID
		}

		report, err := repository.GetReportByFolio(pool, folio, scopeZoneID, citizenID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get report"})
			return
		}

		// El ciudadano no debe ver el análisis de sospecha de su propio reporte
		if citizenID != nil {
			report.SuspiciusLevel = nil
			report.CitizenName = ""
		}

		c.JSON(http.StatusOK, gin.H{"report": report})
	}
}

// DeleteReportHandler maneja DELETE /report/:folio: elimina el reporte, su historial,
// comentarios e imágenes, y después borra las fotos de S3. Solo para personal activo y
// dentro de su zona.
//
// Si alguna foto no se puede borrar de S3 solo se registra en el log. Responde 204.
func DeleteReportHandler(pool *pgxpool.Pool, uploader *S3Uploader) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)
		if !ok {
			return
		}

		folio := strings.TrimSpace(c.Param("folio"))
		if folio == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "you must provide a folio"})
			return
		}

		imageKeys, err := repository.DeleteReport(pool, folio, scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete report"})
			return
		}

		for _, key := range imageKeys {
			if err := uploader.Delete(c.Request.Context(), key); err != nil {
				log.Printf("could not delete image %s of report %s: %v", key, folio, err)
			}
		}

		c.Status(http.StatusNoContent)
	}
}

// UpdateReport maneja PUT /report/:report_id/submit: envía el reporte, es decir, lo
// pasa del borrador al estado "registrado". Falla si alguna de sus imágenes sigue
// pendiente de subir. Una vez enviado, el worker de análisis lo evalúa.
func UpdateReport(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		reportID := c.Param("report_id")

		if reportID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No report ID provided",
			})
			return
		}

		fmt.Println(reportID)

		err := repository.UpdateReportDraft(pool, reportID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err,
			})
			return
		}

		c.JSON(http.StatusOK, true)

	}
}
