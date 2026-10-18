package handlers

import (
	"context"
	"errors"
	"go-service-sipinna/internal/config"
	"go-service-sipinna/internal/middleware"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"go-service-sipinna/internal/resend"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateReport recibe el reporte, el contacto es opcional para seguimiento sin cuenta
type CreateReport struct {
	Description      string  `json:"description"`
	Latitude         float32 `json:"latitude"`
	Longitude        float32 `json:"longitude"`
	ChildrenQuantity *int    `json:"children_quantity" binding:"omitempty,min=1,max=5"`
	ChildrenAge      string  `json:"children_age"`
	WorkType         string  `json:"work_type"`
	SightingTime     string  `json:"sighting_time"`
	Risk             string  `json:"risk" binding:"omitempty,oneof=si no no_se"`
	ImmediateDanger  bool    `json:"immediate_danger"`
	Reference        string  `json:"reference" binding:"max=500"`
}

// CreateReportHandler crea un borrador y devuelve ID, folio y clave de consulta.
// Sin sesión también devuelve edit_token. La zona se asigna por cercanía.
// El reporte se envía con UpdateReport después de subir las fotos.
func CreateReportHandler(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")

		var req CreateReport
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		report := &models.Report{
			Description:      req.Description,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			ChildrenQuantity: req.ChildrenQuantity,
			ChildrenAge:      req.ChildrenAge,
			WorkType:         req.WorkType,
			SightingTime:     req.SightingTime,
			Risk:             req.Risk,
			ImmediateDanger:  req.ImmediateDanger,
			Reference:        strings.TrimSpace(req.Reference),
		}

		var citizenID *string
		var tempKey *uuid.UUID
		if userID != "" {
			citizenID = &userID
		} else {
			key := uuid.New()
			tempKey = &key
		}

		accessKey, err := repository.CreateReport(pool, report, citizenID, tempKey)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		body := gin.H{"reporte_id": report.ID, "folio": report.Folio, "clave_consulta": accessKey}
		if tempKey != nil {
			body["edit_token"] = tempKey.String()
		}
		c.JSON(http.StatusOK, body)

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
		reportID := c.Param("report_ref")

		if reportID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "You must provide a valid report id"})
			return
		}

		if !requireReportEditor(c, pool, reportID) {
			return
		}

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

		// El ciudadano solo ve el estado general
		for i := range reportsBrief {
			reportsBrief[i].State = repository.PublicStatus(reportsBrief[i].State)
		}

		c.JSON(http.StatusOK, gin.H{"reports": reportsBrief})

	}
}

// UpdateReportStatusRequest recibe el estado y el motivo cuando es obligatorio
type UpdateReportStatusRequest struct {
	Status string `json:"estado" binding:"required"`
	Reason string `json:"motivo"`
}

// validReportStatuses son los estados a los que el personal puede mover un reporte
// (valores del enum report_status, excepto DRAFT).
var validReportStatuses = map[string]bool{
	"registrado":      true,
	"en_revision":     true,
	"validado":        true,
	"caso_confirmado": true,
	"canalizado":      true,
	"en_atencion":     true,
	"en_seguimiento":  true,
	"concluido":       true,
	"archivado":       true,
	"cancelado":       true,
	"reincidente":     true,
	"duplicado":       true,
}

// reportStatusesRequiringReason son los estados que exigen un motivo al asignarse.
var reportStatusesRequiringReason = map[string]bool{
	"cancelado":   true,
	"archivado":   true,
	"reincidente": true,
	"duplicado":   true,
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

		folio := strings.TrimSpace(c.Param("report_ref"))

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

func ReportAssignment(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, scopeZoneID, ok := requireActiveStaff(c, pool)

		if !ok {
			return
		}

		folio := strings.TrimSpace(c.Param("report_ref"))

		var req models.ReportAssignment
		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No zone or work type provided"})
			return
		}

		err = repository.UpdateReportAssignment(pool, req.WorkType, req.ZoneID, folio, scopeZoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed while updating"})
			return
		}

		c.JSON(http.StatusOK, true)
	}
}

// requireReportEditor permite editar el reporte a su dueño o, si es anónimo y sigue
// en DRAFT, a quien envíe el header X-Edit-Token igual a temp_key. Si regresa false,
// la respuesta de error ya se escribió y el handler solo debe terminar.
func requireReportEditor(c *gin.Context, pool *pgxpool.Pool, reportID string) bool {
	access, err := repository.GetReportAccess(pool, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not verify report access"})
		return false
	}

	if access.CitizenID != nil {
		userID, err := middleware.UserIDFromRequest(c)
		if errors.Is(err, middleware.ErrJWTNotConfigured) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret is not configured"})
			return false
		}
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return false
		}
		if userID != *access.CitizenID {
			c.JSON(http.StatusForbidden, gin.H{"error": "you can only edit your own reports"})
			return false
		}
		return true
	}

	token := strings.TrimSpace(c.GetHeader("X-Edit-Token"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "edit token is required"})
		return false
	}

	tokenID, err := uuid.Parse(token)
	if err != nil || access.TempKey == nil || access.Status != "DRAFT" || tokenID != *access.TempKey {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid edit token"})
		return false
	}

	return true
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
		folio := strings.TrimSpace(c.Param("report_ref"))
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
			report = &models.ReportDetail{IndividualReport: report.IndividualReport, Images: report.Images}
			report.SuspiciusLevel = nil
			report.CitizenName = ""
			report.LastState = repository.PublicStatus(report.LastState)
			report.Priority = ""
			report.Sex = ""
			report.DuplicateOf = ""
			report.ChildrenCared = nil
			report.CaseID = nil
			report.CaseName = nil
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

		folio := strings.TrimSpace(c.Param("report_ref"))
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

// UpdateReport envía el borrador cuando todas las fotos están listas
// Notifica al personal de la zona
func UpdateReport(pool *pgxpool.Pool, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		reportID := c.Param("report_ref")

		if reportID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No report ID provided",
			})
			return
		}

		if !requireReportEditor(c, pool, reportID) {
			return
		}

		submitted, err := repository.UpdateReportDraft(pool, reportID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		go SendNotifications(pool, cfg, submitted.ZoneID.String(), submitted.ImmediateDanger)

		c.JSON(http.StatusOK, true)

	}
}

func SendNotifications(pool *pgxpool.Pool, cfg *config.Config, zoneID string, urgent bool) {
	alimentadores, err := repository.GetAllAlimentadores(pool, zoneID)
	if err != nil {
		log.Printf("could not list alimentadores for zone %s: %v", zoneID, err)
		return
	}
	for _, alimentador := range alimentadores {
		go func(name, email string) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := resend.SendOneNotification(ctx, cfg, name, email, urgent); err != nil {
				log.Printf("could not notify %s: %v", email, err)
			}
		}(alimentador.Name, alimentador.Email)
	}
}
