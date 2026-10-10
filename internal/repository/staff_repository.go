package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StaffMember es una fila de admins con los datos de su usuario, tal como se muestra
// en el panel de gestión del personal.
type StaffMember struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"nombre"`
	Email           *string    `json:"email"`
	TelephoneNumber *string    `json:"telefono"`
	Role            string     `json:"rol"`
	ZoneID          *uuid.UUID `json:"zona_id"`
	ZoneName        string     `json:"zona_nombre"`
	AccountState    string     `json:"estado_cuenta"`
	CreatedAt       time.Time  `json:"created_at"`
}

// NewStaffMember son los datos para dar de alta personal con [CreateStaffMember].
// HashedPassword ya debe venir con hash bcrypt.
type NewStaffMember struct {
	ID              uuid.UUID
	Name            string
	Email           *string
	TelephoneNumber *string
	HashedPassword  string
	Role            string
	ZoneID          *uuid.UUID
	AccountState    string
}

// staffColumns son las columnas que lee scanStaff, en el mismo orden.
const staffColumns = `
	u.id, COALESCE(u.nombre, ''), u.email, u.telefono, a.rol::text, a.zona_id,
	COALESCE(z.nombre, ''), a.estado_cuenta::text, u.created_at
`

// scanStaff lee una fila con las columnas de staffColumns. Acepta tanto pgx.Row como pgx.Rows.
func scanStaff(row interface{ Scan(...any) error }) (*StaffMember, error) {
	var s StaffMember
	err := row.Scan(&s.ID, &s.Name, &s.Email, &s.TelephoneNumber, &s.Role, &s.ZoneID, &s.ZoneName, &s.AccountState, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListStaff regresa todos los 'administrador' y 'alimentador', primero las cuentas
// pendientes y después por nombre.
func ListStaff(pool *pgxpool.Pool) ([]StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT ` + staffColumns + `
		FROM admins a
		JOIN usuarios u ON u.id = a.id
		LEFT JOIN zonas z ON z.id = a.zona_id
		ORDER BY a.estado_cuenta = 'activada', u.nombre
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	staff := []StaffMember{}
	for rows.Next() {
		s, err := scanStaff(rows)
		if err != nil {
			return nil, err
		}
		staff = append(staff, *s)
	}

	return staff, rows.Err()
}

// GetStaffMember regresa la cuenta de personal id. Regresa pgx.ErrNoRows si no existe.
func GetStaffMember(pool *pgxpool.Pool, id uuid.UUID) (*StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT ` + staffColumns + `
		FROM admins a
		JOIN usuarios u ON u.id = a.id
		LEFT JOIN zonas z ON z.id = a.zona_id
		WHERE a.id = $1
	`

	return scanStaff(pool.QueryRow(ctx, query, id))
}

// CreateStaffMember inserta el usuario y su fila en admins en una sola sentencia y
// regresa la cuenta creada.
func CreateStaffMember(pool *pgxpool.Pool, m NewStaffMember) (*StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		WITH nuevo_usuario AS (
			INSERT INTO usuarios (id, nombre, telefono, email, password_hash)
			VALUES ($1, $2, NULLIF(BTRIM($3), ''), NULLIF(BTRIM($4), ''), $5)
			RETURNING id
		)
		INSERT INTO admins (id, rol, zona_id, estado_cuenta)
		SELECT id, $6::admin_role, $7, $8::account_status FROM nuevo_usuario
	`

	_, err := pool.Exec(ctx, query, m.ID, m.Name, m.TelephoneNumber, m.Email, m.HashedPassword, m.Role, m.ZoneID, m.AccountState)
	if err != nil {
		return nil, err
	}

	return GetStaffMember(pool, m.ID)
}

// UpdateStaffMember cambia rol, zona y estado de la cuenta, y regresa la cuenta
// actualizada. Regresa pgx.ErrNoRows si el id no es personal.
func UpdateStaffMember(pool *pgxpool.Pool, id uuid.UUID, role string, zoneID *uuid.UUID, accountState string) (*StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		UPDATE admins
		SET rol = $2::admin_role, zona_id = $3, estado_cuenta = $4::account_status
		WHERE id = $1
		RETURNING id
	`

	var updatedID uuid.UUID
	if err := pool.QueryRow(ctx, query, id, role, zoneID, accountState).Scan(&updatedID); err != nil {
		return nil, err
	}

	return GetStaffMember(pool, id)
}
