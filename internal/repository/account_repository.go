package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountProfile contiene únicamente los datos públicos de la cuenta.
type AccountProfile struct {
	Name        string  `json:"nombre"`
	Email       *string `json:"email"`
	Phone       *string `json:"telefono"`
	Age         int     `json:"edad"`
	Gender      string  `json:"genero"`
	UserType    string  `json:"user_type"`
	ZoneName    string  `json:"zone_name"`
	Active      bool    `json:"-"`
	HasPassword bool    `json:"has_password"`
}

const profileQuery = `SELECT COALESCE(u.nombre, ''), u.email, u.telefono,
 COALESCE(c.edad, 0), COALESCE(c.genero, ''), COALESCE(a.rol::text, 'citizen'),
 COALESCE(z.nombre, ''), (a.id IS NULL OR a.estado_cuenta = 'activada'),
 COALESCE(u.password_hash, '') <> ''
 FROM usuarios u LEFT JOIN ciudadanos c ON c.id = u.id
 LEFT JOIN admins a ON a.id = u.id LEFT JOIN zonas z ON z.id = a.zona_id WHERE u.id = $1`

func scanProfile(row pgx.Row) (*AccountProfile, error) {
	var p AccountProfile
	err := row.Scan(&p.Name, &p.Email, &p.Phone, &p.Age, &p.Gender, &p.UserType, &p.ZoneName, &p.Active, &p.HasPassword)
	return &p, err
}

func GetAccountProfile(pool *pgxpool.Pool, id uuid.UUID) (*AccountProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return scanProfile(pool.QueryRow(ctx, profileQuery, id))
}

var ErrGoogleEmail = errors.New("el correo de una cuenta de Google no se puede cambiar sin definir primero una contraseña")

// UpdateCitizenProfile actualiza ambas tablas y revoca códigos de recuperación si cambia el contacto.
func UpdateCitizenProfile(pool *pgxpool.Pool, id uuid.UUID, p AccountProfile) (*AccountProfile, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	current, err := scanProfile(tx.QueryRow(ctx, profileQuery+" FOR UPDATE OF u", id))
	if err != nil {
		return nil, err
	}
	if current.UserType != "citizen" {
		return nil, pgx.ErrNoRows
	}
	emailChanged := contactValue(current.Email) != contactValue(p.Email)
	if !current.HasPassword && emailChanged {
		return nil, ErrGoogleEmail
	}
	if _, err = tx.Exec(ctx, `UPDATE usuarios SET nombre=$2, email=$3, telefono=$4 WHERE id=$1`, id, p.Name, p.Email, p.Phone); err != nil {
		return nil, err
	}
	result, err := tx.Exec(ctx, `UPDATE ciudadanos SET edad=$2, genero=$3 WHERE id=$1`, id, p.Age, p.Gender)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() != 1 {
		return nil, pgx.ErrNoRows
	}
	if emailChanged || contactValue(current.Phone) != contactValue(p.Phone) {
		if _, err = tx.Exec(ctx, `DELETE FROM otp_verificaciones WHERE usuario_id=$1`, id); err != nil {
			return nil, err
		}
	}
	updated, err := scanProfile(tx.QueryRow(ctx, profileQuery, id))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func contactValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Las FK conservan reportes, comentarios e historial, y eliminan datos de autenticación.
func DeleteAccount(pool *pgxpool.Pool, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := pool.Exec(ctx, `DELETE FROM usuarios WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}
