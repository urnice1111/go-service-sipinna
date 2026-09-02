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

	var query string = `
		INSERT INTO usuarios 
		(id, nombre, rol, edad, genero, email, telefono, password_hash, zona_id, estado_cuenta)
		VALUES(
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			NULLIF(BTRIM($6), ''),
    		NULLIF(BTRIM($7), ''), 
			$8, 
			$9, 
			$10)
		RETURNING id, nombre, email, created_at, updated_at

	`
	var err = pool.QueryRow(
		ctx,
		query,
		user.ID,
		user.Name,
		user.Role,
		user.Age,
		user.Genre,
		user.Email,
		user.TelephoneNumber,
		user.HashedPassword,
		user.ZoneID,
		user.AccountState).Scan(
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
