package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetAllCases(pool *pgxpool.Pool, scopeZoneID *uuid.UUID) ([]models.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := pool.Query(ctx, `SELECT c.id, c.nombre, COALESCE(c.descripcion, ''), c.zona_id, c.created_at,
 (SELECT COUNT(*) FROM reportes r WHERE r.caso_id = c.id)
 FROM casos c WHERE ($1::uuid IS NULL OR c.zona_id = $1) ORDER BY c.created_at DESC`, scopeZoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := []models.Case{}
	for rows.Next() {
		var c models.Case
		if err := rows.Scan(&c.CaseID, &c.Name, &c.Description, &c.ZoneID, &c.CreatedAt, &c.ReportCount); err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

// CreateCase exige reportes de una sola zona y conserva el alcance municipal.
func CreateCase(pool *pgxpool.Pool, newCase models.CreateCase, scopeZoneID *uuid.UUID) (*models.Case, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT folio, zona_id FROM reportes
 WHERE folio = ANY($1) AND ($2::uuid IS NULL OR zona_id = $2) FOR UPDATE`, newCase.Folios, scopeZoneID)
	if err != nil {
		return nil, err
	}
	var zoneID uuid.UUID
	count := 0
	for rows.Next() {
		var folio string
		var zone uuid.UUID
		if err := rows.Scan(&folio, &zone); err != nil {
			rows.Close()
			return nil, err
		}
		if count > 0 && zone != zoneID {
			rows.Close()
			return nil, pgx.ErrNoRows
		}
		zoneID = zone
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count == 0 || count != len(newCase.Folios) {
		return nil, pgx.ErrNoRows
	}
	c := models.Case{Name: newCase.Name, Description: newCase.Description, ZoneID: &zoneID, ReportCount: count}
	err = tx.QueryRow(ctx, `INSERT INTO casos (nombre, descripcion, zona_id) VALUES ($1, $2, $3) RETURNING id, created_at`, c.Name, c.Description, zoneID).Scan(&c.CaseID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE reportes SET caso_id = $1 WHERE folio = ANY($2)`, c.CaseID, newCase.Folios); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateReportCase(pool *pgxpool.Pool, update models.UpdateCase, folio string, scopeZoneID *uuid.UUID) (*models.UpdateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const query = `WITH updated AS (
 UPDATE reportes r SET caso_id = $1
 WHERE r.folio = $2 AND ($3::uuid IS NULL OR r.zona_id = $3)
 AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM casos c WHERE c.id = $1 AND c.zona_id = r.zona_id))
 RETURNING r.folio, r.caso_id
 ) SELECT r.folio, c.id, c.nombre FROM updated r LEFT JOIN casos c ON r.caso_id = c.id`
	var response models.UpdateResponse
	err := pool.QueryRow(ctx, query, update.CaseID, folio, scopeZoneID).Scan(&response.Folio, &response.CaseID, &response.CaseName)
	if err != nil {
		return nil, err
	}
	return &response, nil
}
