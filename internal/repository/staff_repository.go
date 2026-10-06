package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StaffMember is an admins row with its user data, as shown in the staff management panel.
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

const staffColumns = `
	u.id, COALESCE(u.nombre, ''), u.email, u.telefono, a.rol::text, a.zona_id,
	COALESCE(z.nombre, ''), a.estado_cuenta::text, u.created_at
`

func scanStaff(row interface{ Scan(...any) error }) (*StaffMember, error) {
	var s StaffMember
	err := row.Scan(&s.ID, &s.Name, &s.Email, &s.TelephoneNumber, &s.Role, &s.ZoneID, &s.ZoneName, &s.AccountState, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListStaff returns every 'administrador' and 'alimentador', pending accounts first.
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

// CreateStaffMember inserts the user and its admins row in a single statement.
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

// UpdateStaffMember changes role, zone and account state. Returns pgx.ErrNoRows if the id is not staff.
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
