package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go-service-sipinna/internal/models"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateReport crea un borrador en la zona más cercana y llena r.ID y r.Folio.
// Devuelve la clave de consulta; tempKey permite editar sin sesión.
// El peligro inmediato fija prioridad alta. Sin zona devuelve pgx.ErrNoRows.
func CreateReport(pool *pgxpool.Pool, r *models.Report, ciudadanoID *string, tempKey *uuid.UUID) (uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryCrearReporte = `
	WITH distancias AS (
			SELECT
				id,
				nombre,
				2 * 6371000 * ASIN(
					SQRT(
						LEAST(
							1.0,
							GREATEST(
								0.0,
								POWER(
									SIN(
										RADIANS(latitude - $3::double precision) / 2
									),
									2
								)
								+
								COS(RADIANS($3::double precision))
								* COS(RADIANS(latitude))
								* POWER(
									SIN(
										RADIANS(longitude - $4::double precision) / 2
									),
									2
								)
							)
						)
					)
				) AS distancia_metros
			FROM zonas
			WHERE latitude IS NOT NULL
			AND longitude IS NOT NULL
			AND id IS NOT NULL
	),
	ubicacion_cercana AS (
			SELECT id, nombre
			FROM distancias
			ORDER BY distancia_metros, id
			LIMIT 1
	),
	nuevo_reporte as (
	insert into reportes (
		folio,
		ciudadano_id, 
		descripcion, 
		latitud, 
		longitud, 
		cantidad_ninos, 
		edad_ninos, 
		tipo_trabajo, 
		horario_avistamiento,
		zona_id,
		temp_key,
		riesgo,
		peligro_inmediato,
		prioridad,
		referencia)
	SELECT
				FORMAT(
					'RIETI-%s-%s-%s',
					UPPER(nombre),
					TO_CHAR(CURRENT_DATE, 'YYYY'),
					LPAD(NEXTVAL('reportes_folio_seq')::text, 6, '0')
				),
				$1,
				$2,
				$3::double precision,
				$4::double precision,
				$5,
				$6,
				$7,
				$8,
				id,
				$9,
				NULLIF($10, ''),
				$11,
				CASE WHEN $11 THEN 'alta' ELSE 'normal' END,
				NULLIF($12, '')
			FROM ubicacion_cercana
			RETURNING id, folio, clave_consulta
	)
	SELECT id, folio, clave_consulta FROM nuevo_reporte;
	`

	var accessKey uuid.UUID
	err := pool.QueryRow(
		ctx,
		queryCrearReporte,
		ciudadanoID,
		r.Description,
		r.Latitude,
		r.Longitude,
		r.ChildrenQuantity,
		r.ChildrenAge,
		r.WorkType,
		r.SightingTime,
		tempKey,
		r.Risk,
		r.ImmediateDanger,
		r.Reference,
	).Scan(
		&r.ID,
		&r.Folio,
		&accessKey,
	)

	if err != nil {
		return uuid.Nil, err
	}

	return accessKey, nil

}

// extraReportColumns comparte columnas entre listado y detalle, en orden de escaneo.
const extraReportColumns = `
		COALESCE(z.municipio, '') AS municipio_zona,
		COALESCE(r.horario_avistamiento, '') AS horario_avistamiento,
		COALESCE(r.riesgo, '') AS riesgo,
		r.peligro_inmediato,
		r.prioridad,
		COALESCE(r.sexo, '') AS sexo,
		COALESCE(r.referencia, '') AS referencia,
		COALESCE(dup.folio, '') AS folio_duplicado,
		r.nna_atendidos,
        r.caso_id,
        (SELECT nombre FROM casos WHERE id = r.caso_id) AS nombre_caso`

// reportJoins resuelve zona, ciudadano, posible duplicado y último estado del reporte.
const reportJoins = `
	FROM reportes AS r
	INNER JOIN zonas AS z
		ON r.zona_id = z.id
	-- LEFT: ciudadano_id queda en NULL si se borra el ciudadano (ON DELETE SET NULL).
	LEFT JOIN usuarios AS u
		ON r.ciudadano_id = u.id
	LEFT JOIN reportes AS dup
		ON r.posible_duplicado_de = dup.id
	LEFT JOIN LATERAL (
		SELECT h.estado, h.changed_at
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE`

// reportScanTargets mantiene el mismo orden de campos compartidos entre listado y detalle.
func reportScanTargets(r *models.IndividualReport) []any {
	return []any{
		&r.Folio,
		&r.Description,
		&r.Latitude,
		&r.Longitude,
		&r.ChildrenQuantity,
		&r.WorkType,
		&r.CreatedtAt,
		&r.SuspiciusLevel,
		&r.ChildrenAge,
		&r.ZoneName,
		&r.CitizenName,
		&r.LastState,
		&r.StateChangedAt,
		&r.ZoneMunicipio,
		&r.SightingTime,
		&r.Risk,
		&r.ImmediateDanger,
		&r.Priority,
		&r.Sex,
		&r.Reference,
		&r.DuplicateOf,
		&r.ChildrenCared,
		&r.CaseID,
		&r.CaseName,
	}
}

// GetReportsByZone regresa los reportes filtrados por id de zona o por nombre; un
// zoneID vacío regresa todas las zonas. scopeZoneID, si no es nil, además limita el
// resultado a esa zona (se usa para los usuarios 'alimentador').
//
// Los reportes sin analizar se regresan con nivel de sospecha 0.
func GetReportsByZone(pool *pgxpool.Pool, zoneID string, scopeZoneID *uuid.UUID) ([]models.IndividualReport, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryGetReports = `
	SELECT
		r.folio,
		r.descripcion,
		r.latitud,
		r.longitud,
		r.cantidad_ninos,
		r.tipo_trabajo,
		r.created_at,
		COALESCE(r.sospechoso, 0) AS sospechoso,
		r.edad_ninos,
		z.nombre AS nombre_zona,
		COALESCE(u.nombre, '') AS nombre_ciudadano,
		COALESCE(he.estado::text, 'DRAFT') AS ultimo_estado,
		COALESCE(he.changed_at, r.created_at) AS estado_changed_at,
		` + extraReportColumns + `
` + reportJoins + `
	WHERE
		($1 = '' OR r.zona_id::text = $1 OR UPPER(z.nombre) = UPPER($1))
		AND ($2::uuid IS NULL OR r.zona_id = $2)
		AND he.estado != 'DRAFT';
	`

	rows, err := pool.Query(ctx, queryGetReports, zoneID, scopeZoneID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var AllReports []models.IndividualReport = []models.IndividualReport{}

	for rows.Next() {
		var ir models.IndividualReport
		err = rows.Scan(reportScanTargets(&ir)...)
		if err != nil {
			return nil, err
		}

		AllReports = append(AllReports, ir)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return AllReports, nil
}

// GetReportsSummaryOfUser regresa el resumen de todos los reportes del ciudadano
// userID con su último estado ("DRAFT" si aún no se envía).
func GetReportsSummaryOfUser(pool *pgxpool.Pool, userID string) ([]models.IndividualReportInfoBrief, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)

	defer cancel()

	var query = `
	SELECT
		r.folio,
		COALESCE(he.estado::text, 'DRAFT') AS report_state,
		r.latitud,
		r.longitud,
		r.descripcion,
        r.created_at,
        COALESCE(he.changed_at, r.created_at) AS state_changed_at
	FROM reportes AS r
	LEFT JOIN LATERAL (
		SELECT h.estado, h.changed_at
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE
	WHERE r.ciudadano_id = $1;
	`

	rows, err := pool.Query(ctx, query, userID)

	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[models.IndividualReportInfoBrief])
}

// ErrSameReportStatus lo regresa [UpdateReportStatus] cuando el reporte ya tiene el
// estado solicitado.
var ErrSameReportStatus = errors.New("report already has that status")

// UpdateReportStatus inserta una fila nueva en historial_estados a nombre de adminID;
// el estado actual de un reporte es su fila más reciente de historial_estados. Regresa
// la fecha del cambio.
//
// Regresa pgx.ErrNoRows si el folio no existe o está fuera de scopeZoneID, y
// [ErrSameReportStatus] si el reporte ya tiene ese estado.
func UpdateReportStatus(
	pool *pgxpool.Pool,
	folio string,
	status string,
	reason string,
	adminID uuid.UUID,
	scopeZoneID *uuid.UUID,
) (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)

	// Locking the reportes row serializes concurrent status changes on the same report.
	const queryCurrentStatus = `
		SELECT r.id, COALESCE(he.estado::text, 'DRAFT')
		FROM reportes AS r
		LEFT JOIN LATERAL (
			SELECT h.estado
			FROM historial_estados AS h
			WHERE h.reporte_id = r.id
			ORDER BY h.changed_at DESC, h.id DESC
			LIMIT 1
		) AS he ON TRUE
		WHERE r.folio = $1
			AND ($2::uuid IS NULL OR r.zona_id = $2)
		FOR UPDATE OF r
	`

	var reportID uuid.UUID
	var currentStatus string
	if err := tx.QueryRow(ctx, queryCurrentStatus, folio, scopeZoneID).Scan(&reportID, &currentStatus); err != nil {
		return time.Time{}, err
	}

	if currentStatus == status {
		return time.Time{}, ErrSameReportStatus
	}

	const queryInsertStatus = `
		INSERT INTO historial_estados (reporte_id, estado, cambiado_por, motivo)
		VALUES ($1, $2::report_status, $3, NULLIF($4, ''))
		RETURNING changed_at
	`

	var changedAt time.Time
	if err := tx.QueryRow(ctx, queryInsertStatus, reportID, status, adminID, reason).Scan(&changedAt); err != nil {
		return time.Time{}, err
	}

	const queryTouchReport = `
		UPDATE reportes
		SET updated_at = $1
		WHERE id = $2
	`
	if _, err := tx.Exec(ctx, queryTouchReport, changedAt, reportID); err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return changedAt, nil
}

// GetReportByFolio regresa el detalle del reporte con ese folio, con URLs prefirmadas
// de sus imágenes válidas por 15 minutos.
//
// scopeZoneID limita la búsqueda a una zona (nil = todas) y citizenID a los reportes de
// ese ciudadano (nil = sin límite, para el personal). Regresa pgx.ErrNoRows si el folio
// no existe o queda fuera de esos límites.
func GetReportByFolio(pool *pgxpool.Pool, folio string, scopeZoneID *uuid.UUID, citizenID *uuid.UUID) (*models.ReportDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const queryGetReport = `
	SELECT
		r.id,
		r.folio,
		r.descripcion,
		r.latitud,
		r.longitud,
		r.cantidad_ninos,
		r.tipo_trabajo,
		r.created_at,
		r.sospechoso,
		r.edad_ninos,
		z.nombre AS nombre_zona,
		COALESCE(u.nombre, '') AS nombre_ciudadano,
		COALESCE(he.estado::text, 'DRAFT') AS ultimo_estado,
		COALESCE(he.changed_at, r.created_at) AS estado_changed_at,
		` + extraReportColumns + `,
		COALESCE(r.responsable, ''),
		COALESCE(r.dependencia_canalizada, ''),
		COALESCE(TO_CHAR(r.fecha_seguimiento, 'YYYY-MM-DD'), ''),
		COALESCE(r.resultado, ''),
		COALESCE(TO_CHAR(r.fecha_cierre, 'YYYY-MM-DD'), '')
` + reportJoins + `
	WHERE r.folio = $1
		AND ($2::uuid IS NULL OR r.zona_id = $2)
		AND ($3::uuid IS NULL OR r.ciudadano_id = $3);
	`

	var reportID uuid.UUID
	var rd models.ReportDetail
	targets := append([]any{&reportID}, reportScanTargets(&rd.IndividualReport)...)
	targets = append(targets,
		&rd.Responsible,
		&rd.ChanneledTo,
		&rd.FollowUpDate,
		&rd.Outcome,
		&rd.ClosedDate,
	)
	err := pool.QueryRow(ctx, queryGetReport, folio, scopeZoneID, citizenID).Scan(targets...)
	if err != nil {
		return nil, err
	}

	const queryImages = `
	SELECT url
	FROM imagenes_reporte
	WHERE reporte_id = $1
	ORDER BY orden NULLS LAST, url;
	`

	rows, err := pool.Query(ctx, queryImages, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rd.Images = []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		presignedURL, err := GetPresignedURL("sipinna-photos", url, 15*time.Minute)

		if err != nil {
			return nil, err
		}
		rd.Images = append(rd.Images, presignedURL)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &rd, nil
}

// DeleteReport borra, en una sola transacción, el reporte y sus filas en
// imagenes_reporte, comentarios e historial_estados (no hay FKs con ON DELETE CASCADE).
// Regresa las llaves de las imágenes para que el llamador las borre de S3.
//
// Regresa pgx.ErrNoRows si el folio no existe o está fuera de scopeZoneID.
func DeleteReport(pool *pgxpool.Pool, folio string, scopeZoneID *uuid.UUID) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const queryLockReport = `
		SELECT id
		FROM reportes
		WHERE folio = $1
			AND ($2::uuid IS NULL OR zona_id = $2)
		FOR UPDATE
	`

	var reportID uuid.UUID
	if err := tx.QueryRow(ctx, queryLockReport, folio, scopeZoneID).Scan(&reportID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `DELETE FROM imagenes_reporte WHERE reporte_id = $1 RETURNING url`, reportID)
	if err != nil {
		return nil, err
	}
	imageURLs := []string{}
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			rows.Close()
			return nil, err
		}
		imageURLs = append(imageURLs, url)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM comentarios WHERE reporte_id = $1`, reportID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM historial_estados WHERE reporte_id = $1`, reportID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM reportes WHERE id = $1`, reportID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return imageURLs, nil
}

// WriteImages registra, en una transacción, una fila "pendiente" en imagenes_reporte
// por cada imagen, con una llave única "report/<nombre>.<ext>" y su orden. Regresa los
// ids de las filas en el mismo orden que images_url.Images.
func WriteImages(pool *pgxpool.Pool, reportID string, images_url models.ImagesRequest) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var query string = `
		insert into imagenes_reporte (reporte_id, url, orden)
		values ($1, $2, $3)
		returning id;
	`

	var imagesIDs []string

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	for idx, image := range images_url.Images {
		key := fmt.Sprintf(
			"report/%s",
			generateUniqueFilename(strings.TrimPrefix(image.ContentType, "image/")),
		)

		var tempID string

		err := tx.QueryRow(
			ctx,
			query,
			reportID,
			key,
			idx,
		).Scan(&tempID)

		if err != nil {
			fmt.Println("aqui")
			return nil, err
		}

		imagesIDs = append(imagesIDs, tempID)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return imagesIDs, nil
}

// ======= Helpers

// generateUniqueFilename genera un nombre "<fecha-hora>-<16 hex aleatorios>.<ext>".
func generateUniqueFilename(ext string) string {

	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	timestamp := time.Now().Format("20060102-150405")

	return fmt.Sprintf("%s-%s.%s", timestamp, randomStr, ext)

}

// ReportAccess es lo que se revisa antes de agregar imágenes o enviar un reporte.
type ReportAccess struct {
	CitizenID *uuid.UUID
	TempKey   *uuid.UUID
	Status    string
}

// GetReportAccess regresa el dueño, la llave temporal y el último estado del reporte.
// Regresa pgx.ErrNoRows si el reporte no existe.
func GetReportAccess(pool *pgxpool.Pool, reportID string) (*ReportAccess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
	SELECT
		r.ciudadano_id,
		r.temp_key,
		COALESCE(he.estado::text, 'DRAFT')
	FROM reportes AS r
	LEFT JOIN LATERAL (
		SELECT h.estado
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE
	WHERE r.id = $1
	`

	var access ReportAccess
	err := pool.QueryRow(ctx, query, reportID).Scan(&access.CitizenID, &access.TempKey, &access.Status)
	if err != nil {
		return nil, err
	}

	return &access, nil
}

// UpdateReportDraft registra el envío y borra temp_key si todas las fotos están listas.
// Busca posibles duplicados de la misma zona a unos 100 m durante los últimos 7 días.
// Devuelve error si no existe el reporte o faltan fotos por subir.
func UpdateReportDraft(pool *pgxpool.Pool, reportID string) (*models.SubmittedReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		WITH submitted AS (
			INSERT INTO historial_estados (
				reporte_id,
				estado,
				motivo
			)
			SELECT
				r.id,
				'registrado',
				'Todas las imágenes fueron procesadas'
			FROM reportes r
			WHERE r.id = $1
			AND NOT EXISTS (
				SELECT 1
				FROM imagenes_reporte ir
				WHERE ir.reporte_id = r.id
				  AND ir.estado = 'pendiente'
			)
			RETURNING reporte_id
		)
		UPDATE reportes
		SET temp_key = NULL
		WHERE id IN (SELECT reporte_id FROM submitted)
		RETURNING id, zona_id, folio, clave_consulta, peligro_inmediato, COALESCE(riesgo, '');
	`

	var sr models.SubmittedReport
	err := pool.QueryRow(ctx, query, reportID).Scan(&sr.ID, &sr.ZoneID, &sr.Folio, &sr.AccessKey, &sr.ImmediateDanger, &sr.Risk)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("reporte no encontrado o todavía tiene imágenes pendientes")
	}
	if err != nil {
		return nil, err
	}

	// 0.001° son unos 100 m en el centro de México. Se apunta al reporte original de
	// la cadena para no encadenar duplicados.
	const queryDuplicate = `
		UPDATE reportes AS r
		SET posible_duplicado_de = (
			SELECT COALESCE(o.posible_duplicado_de, o.id)
			FROM reportes AS o
			WHERE o.id <> r.id
				AND o.zona_id = r.zona_id
				AND o.created_at BETWEEN r.created_at - INTERVAL '7 days' AND r.created_at
				AND ABS(o.latitud - r.latitud) < 0.001
				AND ABS(o.longitud - r.longitud) < 0.001
				AND EXISTS (
					SELECT 1 FROM historial_estados AS h
					WHERE h.reporte_id = o.id AND h.estado <> 'DRAFT'
				)
			ORDER BY o.created_at
			LIMIT 1
		)
		WHERE r.id = $1
	`
	if _, err := pool.Exec(ctx, queryDuplicate, reportID); err != nil {
		log.Printf("could not check duplicates of report %s: %v", reportID, err)
	}

	return &sr, nil
}

// UpdateCaseFile modifica el expediente dentro de scopeZoneID.
// Devuelve pgx.ErrNoRows si no es accesible y ErrUnknownDuplicate si falta el original.
func UpdateCaseFile(pool *pgxpool.Pool, folio string, f models.CaseFile, scopeZoneID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if f.DuplicateOf != nil && *f.DuplicateOf != "" {
		var exists bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM reportes WHERE folio = $1 AND folio <> $2 AND ($3::uuid IS NULL OR zona_id = $3))`, *f.DuplicateOf, folio, scopeZoneID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrUnknownDuplicate
		}
	}

	// Para cada campo: NULL = sin cambio, '' = borrar.
	const query = `
		UPDATE reportes
		SET prioridad = COALESCE(NULLIF($1, ''), prioridad),
			sexo = CASE WHEN $2::text IS NULL THEN sexo ELSE NULLIF($2, '') END,
			responsable = CASE WHEN $3::text IS NULL THEN responsable ELSE NULLIF($3, '') END,
			dependencia_canalizada = CASE WHEN $4::text IS NULL THEN dependencia_canalizada ELSE NULLIF($4, '') END,
			fecha_seguimiento = CASE WHEN $5::text IS NULL THEN fecha_seguimiento ELSE NULLIF($5, '')::date END,
			resultado = CASE WHEN $6::text IS NULL THEN resultado ELSE NULLIF($6, '') END,
			nna_atendidos = COALESCE($7, nna_atendidos),
			fecha_cierre = CASE WHEN $8::text IS NULL THEN fecha_cierre ELSE NULLIF($8, '')::date END,
			posible_duplicado_de = CASE
				WHEN $9::text IS NULL THEN posible_duplicado_de
				WHEN $9 = '' THEN NULL
				ELSE (SELECT id FROM reportes WHERE folio = $9 AND folio <> $10 AND ($11::uuid IS NULL OR zona_id = $11))
			END
		WHERE folio = $10
			AND ($11::uuid IS NULL OR zona_id = $11)
	`

	tag, err := pool.Exec(ctx, query,
		f.Priority, f.Sex, f.Responsible, f.ChanneledTo, f.FollowUpDate,
		f.Outcome, f.ChildrenCared, f.ClosedDate, f.DuplicateOf, folio, scopeZoneID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ErrUnknownDuplicate lo regresa [UpdateCaseFile] cuando el folio original no existe.
var ErrUnknownDuplicate = errors.New("duplicate_of folio not found")

// GetReportActions lista acciones en orden cronológico dentro de scopeZoneID.
// Devuelve pgx.ErrNoRows si el reporte no es accesible.
func GetReportActions(pool *pgxpool.Pool, folio string, scopeZoneID *uuid.UUID) ([]models.ReportAction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var reportID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM reportes WHERE folio = $1 AND ($2::uuid IS NULL OR zona_id = $2)`, folio, scopeZoneID).Scan(&reportID)
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT c.id, c.comentario, COALESCE(u.nombre, ''), c.created_at
		FROM comentarios AS c
		LEFT JOIN usuarios AS u ON c.admin_id = u.id
		WHERE c.reporte_id = $1
		ORDER BY c.created_at
	`
	rows, err := pool.Query(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := []models.ReportAction{}
	for rows.Next() {
		var a models.ReportAction
		if err := rows.Scan(&a.ID, &a.Text, &a.StaffName, &a.CreatedAt); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// AddReportAction registra una acción en el reporte folio a nombre de adminID.
// Regresa pgx.ErrNoRows si el folio no existe o está fuera de scopeZoneID.
func AddReportAction(pool *pgxpool.Pool, folio, text string, adminID uuid.UUID, scopeZoneID *uuid.UUID) (*models.ReportAction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		WITH nueva AS (
			INSERT INTO comentarios (reporte_id, admin_id, comentario)
			SELECT id, $3, $4
			FROM reportes
			WHERE folio = $1 AND ($2::uuid IS NULL OR zona_id = $2)
			RETURNING id, comentario, admin_id, created_at
		)
		SELECT n.id, n.comentario, COALESCE(u.nombre, ''), n.created_at
		FROM nueva AS n
		LEFT JOIN usuarios AS u ON n.admin_id = u.id
	`
	var a models.ReportAction
	err := pool.QueryRow(ctx, query, folio, scopeZoneID, adminID, text).Scan(&a.ID, &a.Text, &a.StaffName, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetPublicReportStatus consulta un reporte enviado con folio y clave coincidentes.
// Devuelve el estado interno para traducirlo con PublicStatus; sin coincidencia, pgx.ErrNoRows.
func GetPublicReportStatus(pool *pgxpool.Pool, folio string, accessKey uuid.UUID) (*models.PublicReportStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT r.folio, he.estado::text, r.created_at, he.changed_at
		FROM reportes AS r
		JOIN LATERAL (
			SELECT h.estado, h.changed_at
			FROM historial_estados AS h
			WHERE h.reporte_id = r.id
			ORDER BY h.changed_at DESC, h.id DESC
			LIMIT 1
		) AS he ON TRUE
		WHERE r.folio = $1
			AND r.clave_consulta = $2
			AND he.estado <> 'DRAFT'
	`
	var st models.PublicReportStatus
	err := pool.QueryRow(ctx, query, folio, accessKey).Scan(&st.Folio, &st.Status, &st.CreatedAt, &st.StateChangedAt)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// PublicStatus convierte el estado interno en un estado visible para el ciudadano.
func PublicStatus(status string) string {
	switch status {
	case "registrado":
		return "recibido"
	case "canalizado":
		return "canalizado"
	case "en_atencion", "en_seguimiento":
		return "en_atencion"
	case "concluido", "archivado", "cancelado":
		return "concluido"
	case "DRAFT":
		return "borrador"
	default:
		// en_revision, validado, caso_confirmado, reincidente, duplicado
		return "en_revision"
	}
}

func GetAllAlimentadores(pool *pgxpool.Pool, zoneID string) ([]models.AlimentadorForEmail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
  select u.nombre, u.email from admins a
  inner join usuarios u on a.id = u.id
  where zona_id = $1 and a.estado_cuenta = 'activada';
 `
	rows, err := pool.Query(ctx, query, zoneID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[models.AlimentadorForEmail])
}

// UpdateReportAssignment cambia la categoría y la zona del reporte folio.
// newWorkType o newZoneID nil dejan esa columna como está.
// scopeZoneID nil (administrador) alcanza cualquier reporte; si no es nil, solo el de esa zona.
// Regresa pgx.ErrNoRows si el folio no existe o está fuera de la zona.
func UpdateReportAssignment(pool *pgxpool.Pool, newWorkType *string, newZoneID *string, folio string, scopeZoneID *uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		UPDATE reportes
		SET tipo_trabajo = COALESCE($1, tipo_trabajo),
			zona_id = COALESCE($2::uuid, zona_id)
		WHERE folio = $3
			AND ($4::uuid IS NULL OR zona_id = $4)
	`

	tag, err := pool.Exec(ctx, query, newWorkType, newZoneID, folio, scopeZoneID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetPresignedURL genera una URL prefirmada de GET para objectKey en bucketName,
// válida durante lifetimeDuration. Usa la configuración por defecto de AWS.
func GetPresignedURL(bucketName string, objectKey string, lifetimeDuration time.Duration) (string, error) {
	// 1. Load the default AWS configuration (~/.aws/credentials or environment variables)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("unable to load SDK config: %v", err)
	}

	// 2. Create the S3 client and its dedicated PresignClient
	s3Client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(s3Client)

	// 3. Request a presigned URL for the GetObject operation
	presignedReq, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, s3.WithPresignExpires(lifetimeDuration)) // Set expiration time

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %v", err)
	}

	// 4. Return the complete signed URL string
	return presignedReq.URL, nil
}

// GetReportSuspicion regresa el puntaje de sospecha del reporte (0 = legítimo, 1 = falso) o nil si el análisis no ha terminado
func GetReportSuspicion(ctx context.Context, pool *pgxpool.Pool, reportID uuid.UUID) (*float64, error) {
	var score *float64
	err := pool.QueryRow(ctx, `SELECT sospechoso FROM reportes WHERE id = $1`, reportID).Scan(&score)
	return score, err
}
