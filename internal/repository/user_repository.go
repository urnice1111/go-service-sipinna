package repository

import (
	"context"
	"go-service-sipinna/internal/models"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetUserByContact looks up credentials using either an email or a telephone number.
func GetUserByContact(pool *pgxpool.Pool, email, telephoneNumber string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT id, password_hash
		FROM usuarios
		WHERE (email = NULLIF($1, '') OR telefono = NULLIF($2, ''))
	`
	var user models.User
	err := pool.QueryRow(ctx, query, strings.TrimSpace(email), strings.TrimSpace(telephoneNumber)).Scan(
		&user.ID,
		&user.HashedPassword,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

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

func CreateAdmin(pool *pgxpool.Pool, user *models.User) (*models.User, error) {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const queryCrearAdmin = `
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
		nuevo_admin AS (
			INSERT INTO admins (id, rol)
			SELECT id, $6 FROM nuevo_usuario
		)
		SELECT id, nombre, email, created_at, updated_at FROM nuevo_usuario;
	`

	var err = pool.QueryRow(
		ctx,
		queryCrearAdmin,
		user.ID,
		user.Name,
		user.TelephoneNumber,
		user.Email,
		user.HashedPassword,
		user.Role,
	).Scan(
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
