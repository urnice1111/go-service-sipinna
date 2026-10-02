package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateReport(pool *pgxpool.Pool, r *models.Report, ciudadanoID *string) (*models.Report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryCrearReporte = `
	WITH distancias AS (
			SELECT
				id,
				municipio,
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
			SELECT id, municipio
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
		zona_id)
	SELECT
				FORMAT(
					'RIETI-%s-%s-%s',
					UPPER(municipio),
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
				id
			FROM ubicacion_cercana
			RETURNING id
	)
	SELECT id FROM nuevo_reporte;
	`

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
	).Scan(
		&r.ID,
	)

	if err != nil {
		return nil, err
	}

	return r, nil

}

// GetReportsByZone filters by zone id or municipio; an empty zoneID returns every zone. scopeZoneID, when not nil, also
// restricts the result to that zone (used for 'alimentador' users).
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
		COALESCE(he.changed_at, r.created_at) AS estado_changed_at
	FROM reportes AS r
	INNER JOIN zonas AS z
		ON r.zona_id = z.id
	-- LEFT: ciudadano_id queda en NULL si se borra el ciudadano (ON DELETE SET NULL).
	LEFT JOIN usuarios AS u
		ON r.ciudadano_id = u.id
	LEFT JOIN LATERAL (
		SELECT h.estado, h.changed_at
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE
	WHERE
		($1 = '' OR r.zona_id::text = $1 OR UPPER(z.municipio) = UPPER($1))
		AND ($2::uuid IS NULL OR r.zona_id = $2);
	`

	rows, err := pool.Query(ctx, queryGetReports, zoneID, scopeZoneID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var AllReports []models.IndividualReport = []models.IndividualReport{}

	for rows.Next() {
		var ir models.IndividualReport
		err = rows.Scan(
			&ir.Folio,
			&ir.Description,
			&ir.Latitude,
			&ir.Longitude,
			&ir.ChildrenQuantity,
			&ir.WorkType,
			&ir.CreatedtAt,
			&ir.SuspiciusLevel,
			&ir.ChildrenAge,
			&ir.ZoneName,
			&ir.CitizenName,
			&ir.LastState,
			&ir.StateChangedAt,
		)
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

func GetReportsSummaryOfUser(pool *pgxpool.Pool, userID string) ([]models.IndividualReportInfoBrief, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)

	defer cancel()

	var query = `
	SELECT
		r.folio,
		COALESCE(he.estado::text, 'DRAFT') AS report_state,
		r.latitud,
		r.longitud,
		r.descripcion
	FROM reportes AS r
	LEFT JOIN LATERAL (
		SELECT h.estado
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

	defer rows.Close()

	var AllReportsSummary []models.IndividualReportInfoBrief = []models.IndividualReportInfoBrief{}

	for rows.Next() {
		var tempReport models.IndividualReportInfoBrief

		err = rows.Scan(
			&tempReport.Folio,
			&tempReport.State,
			&tempReport.Latitude,
			&tempReport.Longitude,
			&tempReport.Description,
		)
		if err != nil {
			return nil, err
		}
		AllReportsSummary = append(AllReportsSummary, tempReport)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return AllReportsSummary, nil

}

var ErrSameReportStatus = errors.New("report already has that status")

// UpdateReportStatus inserts a new row in historial_estados; the current status of a report
// is its most recent historial_estados row. Returns pgx.ErrNoRows when the folio does not
// exist or is outside scopeZoneID.
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

	if _, err := tx.Exec(ctx, `UPDATE reportes SET updated_at = $1 WHERE id = $2`, changedAt, reportID); err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return changedAt, nil
}

// GetReportByFolio returns pgx.ErrNoRows when the folio does not exist or is outside scopeZoneID.
func GetReportByFolio(pool *pgxpool.Pool, folio string, scopeZoneID *uuid.UUID) (*models.ReportDetail, error) {
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
		COALESCE(r.horario_avistamiento, '') AS horario_avistamiento
	FROM reportes AS r
	INNER JOIN zonas AS z
		ON r.zona_id = z.id
	LEFT JOIN usuarios AS u
		ON r.ciudadano_id = u.id
	LEFT JOIN LATERAL (
		SELECT h.estado, h.changed_at
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE
	WHERE r.folio = $1
		AND ($2::uuid IS NULL OR r.zona_id = $2);
	`

	var reportID uuid.UUID
	var rd models.ReportDetail
	err := pool.QueryRow(ctx, queryGetReport, folio, scopeZoneID).Scan(
		&reportID,
		&rd.Folio,
		&rd.Description,
		&rd.Latitude,
		&rd.Longitude,
		&rd.ChildrenQuantity,
		&rd.WorkType,
		&rd.CreatedtAt,
		&rd.SuspiciusLevel,
		&rd.ChildrenAge,
		&rd.ZoneName,
		&rd.CitizenName,
		&rd.LastState,
		&rd.StateChangedAt,
		&rd.SightingTime,
	)
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
		rd.Images = append(rd.Images, url)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &rd, nil
}

// DeleteReport removes the report and its rows in imagenes_reporte, comentarios and
// historial_estados (there are no FKs with ON DELETE CASCADE). Returns the image urls so
// the caller can remove them from S3. Returns pgx.ErrNoRows when the folio does not exist
// or is outside scopeZoneID.
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
			generateUniqueFilename(image.ContentType),
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
func generateUniqueFilename(ext string) string {

	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	timestamp := time.Now().Format("20060102-150405")

	return fmt.Sprintf("%s-%s.%s", timestamp, randomStr, ext)

}

func UpdateReportDraft(pool *pgxpool.Pool, reportID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
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
		AND EXISTS (
			SELECT 1
			FROM imagenes_reporte ir
			WHERE ir.reporte_id = r.id
		)
		AND NOT EXISTS (
			SELECT 1
			FROM imagenes_reporte ir
			WHERE ir.reporte_id = r.id
			  AND ir.estado = 'pendiente'
		);
	`

	result, err := pool.Exec(ctx, query, reportID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("reporte no encontrado o todavía tiene imágenes pendientes")
	}

	return nil
}
