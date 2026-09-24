package repository

import (
	"context"
	"errors"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateReport(pool *pgxpool.Pool, r *models.Report, ps []string, ciudadanoID *string) (*models.Report, error) {
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
									RADIANS(latitude - $4::double precision) / 2
								),
								2
							)
							+
							COS(RADIANS($4::double precision))
							* COS(RADIANS(latitude))
							* POWER(
								SIN(
									RADIANS(longitude - $5::double precision) / 2
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
	nuevo_reporte AS (
		INSERT INTO reportes (
			id,
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
			fecha_eliminacion_programada
		)
		SELECT
			$1,
			FORMAT(
				'RIETI-%s-%s-%s',
				UPPER(municipio),
				TO_CHAR(CURRENT_DATE, 'YYYY'),
				LPAD(NEXTVAL('reportes_folio_seq')::text, 6, '0')
			),
			$2,
			$3,
			$4::double precision,
			$5::double precision,
			$6,
			$7,
			$8,
			$9,
			id,
			$10
		FROM ubicacion_cercana
		RETURNING id, zona_id, folio, created_at
	),
	nuevas_imagenes AS (
		INSERT INTO imagenes_reporte (
			reporte_id,
			url
		)
		SELECT
			reporte.id,
			imagen.url
		FROM nuevo_reporte AS reporte
		CROSS JOIN UNNEST($11::text[]) AS imagen(url)
	)
	SELECT id, folio, zona_id, created_at
	FROM nuevo_reporte;
	`

	err := pool.QueryRow(
		ctx,
		queryCrearReporte,
		r.ID,
		ciudadanoID,
		r.Description,
		r.Latitude,
		r.Longitude,
		r.ChildrenQuantity,
		r.ChildrenAge,
		r.WorkType,
		r.SightingTime,
		r.DeleteDate,
		ps,
	).Scan(
		&r.ID,
		&r.Folio,
		&r.ZoneID,
		&r.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("Thers no valid zones")
	}

	if err != nil {
		return nil, err
	}

	return r, nil

}

// GetReportsByZone filters by zone id or municipio. scopeZoneID, when not nil, also
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
		u.nombre AS nombre_ciudadano,
		r.estado::text AS ultimo_estado,
		COALESCE(he.changed_at, r.created_at) AS estado_changed_at
	FROM reportes AS r
	INNER JOIN zonas AS z
		ON r.zona_id = z.id
	INNER JOIN usuarios AS u
		ON r.ciudadano_id = u.id
	LEFT JOIN LATERAL (
		SELECT h.changed_at
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	) AS he ON TRUE
	WHERE
		(r.zona_id::text = $1 OR UPPER(z.municipio) = UPPER($1))
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
		r.estado::text AS report_state,
		r.latitud,
		r.longitud,
		r.descripcion
	FROM reportes AS r
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

// UpdateReportStatus changes reportes.estado. The historial_estados row is written by the
// log_report_status_change trigger, which reads app.admin_id and app.motivo from this
// transaction. Returns pgx.ErrNoRows when the folio does not exist or is outside scopeZoneID.
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

	const queryCurrentStatus = `
		SELECT estado::text
		FROM reportes
		WHERE folio = $1
			AND ($2::uuid IS NULL OR zona_id = $2)
		FOR UPDATE
	`

	var currentStatus string
	if err := tx.QueryRow(ctx, queryCurrentStatus, folio, scopeZoneID).Scan(&currentStatus); err != nil {
		return time.Time{}, err
	}

	if currentStatus == status {
		return time.Time{}, ErrSameReportStatus
	}

	const querySetContext = `
		SELECT
			set_config('app.admin_id', $1, true),
			set_config('app.motivo', $2, true)
	`

	if _, err := tx.Exec(ctx, querySetContext, adminID.String(), reason); err != nil {
		return time.Time{}, err
	}

	const queryUpdateStatus = `
		UPDATE reportes
		SET estado = $1::report_status,
			updated_at = CURRENT_TIMESTAMP
		WHERE folio = $2
		RETURNING updated_at
	`

	var changedAt time.Time
	if err := tx.QueryRow(ctx, queryUpdateStatus, status, folio).Scan(&changedAt); err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}

	return changedAt, nil
}
