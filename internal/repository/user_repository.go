package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateUser(pool *pgxpool.Pool, user *models.User) (*models.User, error) {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryCrearCiudadano = `
	WITH nuevo_usuario AS (
		INSERT INTO usuarios (id, nombre, telefono, email, password_hash)
		VALUES (
			$1,
			$2,
			NULLIF(BTRIM($3), ''),
			NULLIF(BTRIM($4), ''),
			$5
		)
		RETURNING id, nombre, email, created_at, updated_at
	),
	nuevo_ciudadano AS (
		INSERT INTO ciudadanos (id, edad, genero)
		SELECT id, $6, $7 FROM nuevo_usuario
	)
	SELECT id, nombre, email, created_at, updated_at FROM nuevo_usuario;
	`

	var err = pool.QueryRow(
		ctx,
		queryCrearCiudadano,
		user.ID,
		user.Name,
		user.TelephoneNumber,
		user.Email,
		user.HashedPassword,
		user.Age,
		user.Genre).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return user, nil

}
