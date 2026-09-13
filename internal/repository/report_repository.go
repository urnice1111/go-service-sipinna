package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateReport(pool *pgxpool.Pool, report *models.Report) (*models.Report, error) {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	var query string = `
		INSERT INTO reportes
		(id, folio, usuario_id, descripcion, latitud, longitud, direccion,
		 cantidad_ninos, edad_ninos, tipo_trabajo, horario_avistamiento, condicion, estado)
		VALUES(
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			NULLIF(BTRIM($7), ''),
			$8,
			$9,
			$10,
			$11,
			$12,
			$13)
		RETURNING id, folio, estado, created_at
	`

	var err = pool.QueryRow(
		ctx,
		query,
		report.ID,
		report.Folio,
		report.UserID,
		report.Description,
		report.Latitude,
		report.Longitude,
		report.Address,
		report.ChildrenCount,
		report.ChildrenAge,
		report.WorkType,
		report.SightingTime,
		report.Condition,
		report.State).Scan(
		&report.ID,
		&report.Folio,
		&report.State,
		&report.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return report, nil
}
