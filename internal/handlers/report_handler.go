package handlers

import (
	"context"
	"errors"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateReportHandler(pool *pgxpool.Pool, r *models.Report, ps []string, ciudadanoID *string) (*models.Report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryCrearReporte = `
	WITH distancias AS (
		SELECT
			id,
			2 * 6371000 * ASIN(
				SQRT(
					LEAST(1.0, GREATEST(0.0,
						POWER(
							SIN(
								RADIANS(latitude - $5::double precision) / 2
							),
							2
						)
						+
						COS(RADIANS($5::double precision))
						* COS(RADIANS(latitude))
						* POWER(
							SIN(
								RADIANS(longitude - $6::double precision) / 2
							),
							2
						)
					))
				)
			) AS distancia_metros
		FROM zonas
		WHERE latitude IS NOT NULL
		AND longitude IS NOT NULL
		AND id IS NOT NULL
	),
	ubicacion_cercana AS (
		SELECT id
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
			$2,
			$3,
			$4,
			$5::double precision,
			$6::double precision,
			$7,
			$8,
			$9,
			$10
			zona_id,
			$11
		FROM ubicacion_cercana
		RETURNING id, zona_id, created_at
	),
	nuevas_imagenes AS (
		INSERT INTO reporte_imagenes (
			reporte_id,
			url
		)
		SELECT
			reporte.id,
			imagen.url
		FROM nuevo_reporte AS reporte
		CROSS JOIN UNNEST($5::text[]) AS imagen(url)
	)
	SELECT id, zona_id, created_at
	FROM nuevo_reporte;
	`

	err := pool.QueryRow(
		ctx,
		queryCrearReporte,
		r.ID,
		r.Folio,
		ciudadanoID,
		r.Description,
		r.Latitude,
		r.Longitude,
		r.ChildrenQuantity,
		r.ChildrenAge,
		r.WorkType,
		r.SightingTime,
		r.DeleteDate,
	).Scan(
		&r.ID,
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
