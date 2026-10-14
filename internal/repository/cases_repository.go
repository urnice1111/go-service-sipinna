package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetAllCases(pool *pgxpool.Pool) ([]models.Case, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	var query = `
		select id, nombre, descripcion from casos;
	`

	rows, err := pool.Query(ctx, query)

	if err != nil {
		return nil, err
	}

	var cases []models.Case
	for rows.Next() {

		var temp models.Case
		err := rows.Scan(
			&temp.CaseID,
			&temp.Name,
			&temp.Description,
		)

		if err != nil {
			return nil, err
		}

		cases = append(cases, temp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return cases, nil

}

func CreateCase(pool *pgxpool.Pool, newCase models.CreateCase) (*uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var query = `
	WITH inserted_case AS (
				INSERT INTO casos (nombre, descripcion)
				VALUES ($1, $2)
				RETURNING id
			),
	updated_reports as (
	UPDATE reportes
			SET caso_id = (SELECT id from inserted_case)
			WHERE folio = ANY($3)
			returning 1
	)
	select id from inserted_case;
	`

	var caseID uuid.UUID
	err := pool.QueryRow(ctx, query, newCase.Name, newCase.Description, newCase.Folios).Scan(
		&caseID,
	)

	if err != nil {
		return nil, err
	}

	return &caseID, nil

}

func UpdateReportCase(pool *pgxpool.Pool, update models.UpdateCase, folio string) (*models.UpdateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var query = `
	with updated_case as (
		update reportes
		set caso_id = $1
		where folio = $2
	) 
	select r.folio, c.id as case_id, c.nombre as case_name from reportes r
	inner join casos c on r.caso_id = c.id
	where r.folio = $2;
	`

	var response models.UpdateResponse

	err := pool.QueryRow(ctx, query, update.CaseID, folio).Scan(
		&response.Folio,
		&response.CaseID, //order must coincide lol
		&response.CaseName,
	)

	if err != nil {
		return nil, err
	}

	return &response, nil

}
