package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// invitationOTPType es el valor de otp_verificaciones.tipo para las invitaciones de
// personal.
const invitationOTPType = "invitacion_admin"

// ErrInvalidInvitation indica que el token no existe, venció, ya se usó o la cuenta
// ya no está esperando aceptar su invitación.
var ErrInvalidInvitation = errors.New("invalid or expired invitation")

// pendingInvitationQuery selecciona el usuario de una invitación vigente cuyo hash es
// $1. Una cuenta espera su invitación mientras está 'pendiente' y sin contraseña; una
// cuenta suspendida también es 'pendiente', pero ya tiene contraseña.
const pendingInvitationQuery = `
	SELECT o.id, o.usuario_id
	FROM otp_verificaciones o
	JOIN usuarios u ON u.id = o.usuario_id
	JOIN admins a ON a.id = o.usuario_id
	WHERE o.codigo_hash = $1
		AND o.tipo = '` + invitationOTPType + `'
		AND o.estado = 'pendiente'
		AND o.expira_at > now()
		AND a.estado_cuenta = 'pendiente'
		AND u.password_hash IS NULL
`

// CreateInvitation guarda tokenHash como la invitación vigente de userID durante ttl
// y reemplaza las invitaciones pendientes anteriores.
func CreateInvitation(pool *pgxpool.Pool, userID uuid.UUID, tokenHash string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const queryReplace = `
		UPDATE otp_verificaciones
		SET estado = 'reemplazado'
		WHERE usuario_id = $1
			AND tipo = $2
			AND estado = 'pendiente'
	`
	if _, err := tx.Exec(ctx, queryReplace, userID, invitationOTPType); err != nil {
		return err
	}

	const queryInsert = `
		INSERT INTO otp_verificaciones (usuario_id, codigo_hash, tipo, estado, expira_at)
		VALUES ($1, $2, $3, 'pendiente', now() + make_interval(secs => $4))
	`
	if _, err := tx.Exec(ctx, queryInsert, userID, tokenHash, invitationOTPType, ttl.Seconds()); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetInvitation regresa la cuenta invitada con el token tokenHash. Regresa
// [ErrInvalidInvitation] si la invitación no es válida.
func GetInvitation(pool *pgxpool.Pool, tokenHash string) (*StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var otpID, userID uuid.UUID
	err := pool.QueryRow(ctx, pendingInvitationQuery, tokenHash).Scan(&otpID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidInvitation
	}
	if err != nil {
		return nil, err
	}

	return GetStaffMember(pool, userID)
}

// AcceptInvitation guarda passwordHash como contraseña de la cuenta invitada con el
// token tokenHash, la activa y marca el token como usado. Regresa la cuenta activada,
// o [ErrInvalidInvitation] si la invitación no es válida.
func AcceptInvitation(pool *pgxpool.Pool, tokenHash, passwordHash string) (*StaffMember, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var otpID, userID uuid.UUID
	err = tx.QueryRow(ctx, pendingInvitationQuery+` FOR UPDATE OF o, u, a`, tokenHash).Scan(&otpID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidInvitation
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash = $1, updated_at = now() WHERE id = $2`, passwordHash, userID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE admins SET estado_cuenta = 'activada' WHERE id = $1`, userID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE otp_verificaciones SET estado = 'usado' WHERE id = $1`, otpID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return GetStaffMember(pool, userID)
}

// RejectInvitation borra la cuenta invitada con el token tokenHash. Regresa
// [ErrInvalidInvitation] si la invitación no es válida.
func RejectInvitation(pool *pgxpool.Pool, tokenHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var otpID, userID uuid.UUID
	err = tx.QueryRow(ctx, pendingInvitationQuery+` FOR UPDATE OF o, u, a`, tokenHash).Scan(&otpID, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidInvitation
	}
	if err != nil {
		return err
	}

	if err := deleteInvitedStaff(ctx, tx, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DeleteInvitedStaff borra la cuenta userID si todavía espera aceptar su invitación.
// Se usa cuando el correo de invitación no se pudo enviar.
func DeleteInvitedStaff(pool *pgxpool.Pool, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := deleteInvitedStaff(ctx, tx, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// deleteInvitedStaff borra, en el orden que piden las llaves foráneas, los tokens, la
// fila de admins y el usuario userID, solo si la cuenta sigue sin aceptar su invitación.
func deleteInvitedStaff(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const queryCheck = `
		SELECT 1
		FROM usuarios u
		JOIN admins a ON a.id = u.id
		WHERE u.id = $1
			AND a.estado_cuenta = 'pendiente'
			AND u.password_hash IS NULL
		FOR UPDATE OF u, a
	`
	var one int
	if err := tx.QueryRow(ctx, queryCheck, userID).Scan(&one); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM otp_verificaciones WHERE usuario_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM admins WHERE id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM usuarios WHERE id = $1`, userID); err != nil {
		return err
	}
	return nil
}

// IsInvitationPending indica si userID es personal que todavía no acepta su
// invitación. Los ciudadanos sin contraseña (creados con Google) regresan false.
func IsInvitationPending(pool *pgxpool.Pool, userID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM usuarios u
			JOIN admins a ON a.id = u.id
			WHERE u.id = $1
				AND a.estado_cuenta = 'pendiente'
				AND u.password_hash IS NULL
		)
	`

	var pending bool
	err := pool.QueryRow(ctx, query, userID).Scan(&pending)
	return pending, err
}
