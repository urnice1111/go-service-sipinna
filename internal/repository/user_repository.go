package repository

import (
	"context"
	"fmt"
	"go-service-sipinna/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserType struct {
	UserType string
}

// StaffAccess is an admins row with an activated account ('administrador' or 'alimentador').
type StaffAccess struct {
	Role   string
	ZoneID *uuid.UUID
}

// GetActiveStaff returns pgx.ErrNoRows when the user is not in admins or its account is not activated.
func GetActiveStaff(pool *pgxpool.Pool, userID uuid.UUID) (*StaffAccess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT rol::text, zona_id
		FROM admins
		WHERE id = $1
			AND estado_cuenta = 'activada'
	`

	var staff StaffAccess
	if err := pool.QueryRow(ctx, query, userID).Scan(&staff.Role, &staff.ZoneID); err != nil {
		return nil, err
	}

	return &staff, nil
}

// GetUserByContact looks up credentials using either an email or a telephone number.
func GetUserByContact(pool *pgxpool.Pool, email, telephoneNumber string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT id, password_hash, nombre
		FROM usuarios
		WHERE (email = NULLIF($1, '') OR telefono = NULLIF($2, ''))
	`
	var user models.User
	err := pool.QueryRow(ctx, query, strings.TrimSpace(email), strings.TrimSpace(telephoneNumber)).Scan(
		&user.ID,
		&user.HashedPassword,
		&user.Name,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserType también regresa el nombre de la zona asignada; vacío para ciudadanos o staff sin zona.
func GetUserType(
	pool *pgxpool.Pool,
	userID uuid.UUID) (string, bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `	SELECT
			COALESCE(a.rol::text, 'citizen'),
			COALESCE(a.rol = 'administrador' OR a.rol = 'alimentador' AND a.estado_cuenta = 'activada', false),
			COALESCE(z.nombre, '')
		FROM usuarios u
		LEFT JOIN admins a ON a.id = u.id
		LEFT JOIN zonas z ON z.id = a.zona_id
		WHERE u.id = $1`

	var userType string
	var isAdmin bool
	var zoneName string

	if err := pool.QueryRow(ctx, query, userID).Scan(&userType, &isAdmin, &zoneName); err != nil {
		return "", false, "", fmt.Errorf("get user type: %w", err)
	}

	fmt.Println(isAdmin)
	return userType, isAdmin, zoneName, nil
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

func ActivateAdminAccount(pool *pgxpool.Pool, adminID uuid.UUID) (string, error) {
	var ctx context.Context
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	const query = `
	UPDATE admins
	SET estado_cuenta = 'activada'
	WHERE id = $1
		AND estado_cuenta = 'pendiente'
	RETURNING estado_cuenta
	`
	var accountStatus string
	err := pool.QueryRow(ctx, query, adminID).Scan(&accountStatus)

	if err != nil {
		return "", err
	}

	return accountStatus, nil

}
