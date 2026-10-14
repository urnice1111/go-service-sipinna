package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertReportElevenLabs(pool *pgxpool.Pool, coordinates *models.GeoCode, newReport *models.ElevenLabsRequest) (*models.ElevenLabsRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	var query = `
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
								SIN(RADIANS(latitude - $2::double precision) / 2),
								2
							)
							+
							COS(RADIANS($2::double precision))
							* COS(RADIANS(latitude))
							* POWER(
								SIN(RADIANS(longitude - $3::double precision) / 2),
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
	zona_cercana AS (
		SELECT municipio, id
		FROM distancias
		ORDER BY distancia_metros ASC
		LIMIT 1
	),
	new_report AS (
		INSERT INTO reportes (
			folio,
			descripcion,
			latitud,
			longitud,
			cantidad_ninos,
			edad_ninos,
			tipo_trabajo,
			horario_avistamiento,
			zona_id
		)
		SELECT
			FORMAT(
				'RIETI-%s-%s-%s',
				UPPER(municipio),
				TO_CHAR(CURRENT_DATE, 'YYYY'),
				LPAD(NEXTVAL('reportes_folio_seq')::text, 6, '0')
			),
			$1, $2, $3, $4, $5, $6, $7, id
		FROM zona_cercana
		RETURNING id
	),
	new_state AS (
		INSERT INTO historial_estados (
			reporte_id,
			estado,
			motivo
		)
		SELECT
			id,
			'registrado',
			'Registrado desde ElevenLabs'
		FROM new_report
		RETURNING reporte_id
	)
	SELECT id FROM new_report;
	`

	err := pool.QueryRow(
		ctx,
		query,
		newReport.Description,
		coordinates.Latitude,
		coordinates.Longitude,
		newReport.NNAQuantity,
		newReport.NNAAge,
		newReport.WorkType,
		newReport.SightingTime).Scan(&newReport.ReportID)

	if err != nil {
		return nil, err
	}

	return newReport, nil
}
