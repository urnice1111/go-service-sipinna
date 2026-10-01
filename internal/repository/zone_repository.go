package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetZones returns every zone ordered by name. scopeZoneID, when not nil, restricts the
// result to that zone (used for 'alimentador' users).
func GetZones(pool *pgxpool.Pool, scopeZoneID *uuid.UUID) ([]models.Zone, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT id, nombre, COALESCE(municipio, ''), latitude, longitude
		FROM zonas
		WHERE $1::uuid IS NULL OR id = $1
		ORDER BY nombre
	`

	rows, err := pool.Query(ctx, query, scopeZoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	zones := []models.Zone{}

	for rows.Next() {
		var z models.Zone
		if err := rows.Scan(&z.ID, &z.Name, &z.Municipio, &z.Latitude, &z.Longitude); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return zones, nil
}
