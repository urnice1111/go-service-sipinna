package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func GetImage(pool *pgxpool.Pool, reportID string, imageID string) (*models.GetImage, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		select ir.url, ir.estado from reportes r 
		inner join imagenes_reporte ir on r.id = ir.reporte_id and ir.id = $1 and r.id = $2; 
	`

	var image models.GetImage
	err := pool.QueryRow(ctx, query, imageID, reportID).Scan(&image.URL, &image.Status)

	if err != nil {
		return nil, err
	}

	return &image, nil
}
