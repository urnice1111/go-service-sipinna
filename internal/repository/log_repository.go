package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InsertLogEntry registra una petición del usuario en la bitácora.
func InsertLogEntry(pool *pgxpool.Pool, userID uuid.UUID, method, path string, status int, ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx,
		`INSERT INTO bitacora (usuario_id, metodo, ruta, estatus, ip) VALUES ($1, $2, $3, $4, $5)`,
		userID, method, path, status, ip)
	return err
}

// GetLogEntries devuelve hasta limit accesos, del más reciente al más antiguo.
func GetLogEntries(pool *pgxpool.Pool, limit int) ([]models.LogEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT COALESCE(u.nombre, ''), b.metodo, b.ruta, b.estatus, COALESCE(b.ip, ''), b.created_at
		FROM bitacora AS b
		LEFT JOIN usuarios AS u ON b.usuario_id = u.id
		ORDER BY b.created_at DESC
		LIMIT $1
	`
	rows, err := pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []models.LogEntry{}
	for rows.Next() {
		var e models.LogEntry
		if err := rows.Scan(&e.UserName, &e.Method, &e.Path, &e.Status, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
