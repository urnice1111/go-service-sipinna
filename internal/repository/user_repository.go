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

// UserType es el tipo de usuario ("citizen", "administrador" o "alimentador").
type UserType struct {
	UserType string
}

// StaffAccess es una fila de admins con la cuenta activada ('administrador' o
// 'alimentador'). ZoneID es nil cuando no tiene zona asignada.
type StaffAccess struct {
	Role   string
	ZoneID *uuid.UUID
}

// GetActiveStaff regresa el rol y la zona del personal userID. Regresa pgx.ErrNoRows
// si el usuario no está en admins o su cuenta no está activada.
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

// GetUserByContact busca un usuario por correo o por teléfono (un valor vacío se
// ignora). Solo llena ID, HashedPassword y Name. Regresa pgx.ErrNoRows si no existe.
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

// GetUserType regresa el tipo de usuario ("citizen" si no está en admins), si es
// personal con cuenta activada (isAdmin) y el nombre de la zona asignada; este último
// queda vacío para ciudadanos o staff sin zona.
func GetUserType(
	pool *pgxpool.Pool,
	userID uuid.UUID) (string, bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `	SELECT
			COALESCE(a.rol::text, 'citizen'),
			COALESCE((a.rol = 'administrador' OR a.rol = 'alimentador') AND a.estado_cuenta = 'activada', false),
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

	return userType, isAdmin, zoneName, nil
}

// CreateUser inserta un ciudadano (filas en usuarios y ciudadanos) en una sola
// sentencia. Llena en user las fechas de creación y actualización.
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

// CreateAdmin inserta una cuenta de personal (filas en usuarios y admins) con el rol
// user.Role. La cuenta queda con el estado por defecto de la tabla (pendiente).
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

// ActivateAdminAccount cambia la cuenta de personal adminID de "pendiente" a
// "activada" y regresa el nuevo estado. Regresa pgx.ErrNoRows si la cuenta no existe o
// no estaba pendiente.
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
