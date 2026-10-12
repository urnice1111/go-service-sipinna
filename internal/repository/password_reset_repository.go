package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// passwordResetOTPType es el valor de otp_verificaciones.tipo para los códigos de
// recuperación de contraseña.
const passwordResetOTPType = "recuperar_password"

// MaxPasswordResetAttempts es cuántas veces se puede equivocar el código antes de
// que se bloquee y haya que pedir otro.
const MaxPasswordResetAttempts = 5

// ErrResetCodeTooSoon indica que el usuario pidió otro código antes de que pasara
// el tiempo mínimo entre envíos.
var ErrResetCodeTooSoon = errors.New("a reset code was requested recently")

// ErrInvalidResetCode indica que el código no existe, venció, ya se usó, se bloqueó
// por intentos o no coincide.
var ErrInvalidResetCode = errors.New("invalid or expired reset code")

// CreatePasswordResetCode guarda codeHash como el código de recuperación vigente de
// userID durante ttl y reemplaza los códigos pendientes anteriores. Regresa
// [ErrResetCodeTooSoon] si ya se creó uno hace menos de cooldown.
func CreatePasswordResetCode(pool *pgxpool.Pool, userID uuid.UUID, codeHash string, ttl, cooldown time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Locking the usuarios row serializes concurrent requests for the same user.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM usuarios WHERE id = $1 FOR UPDATE`, userID); err != nil {
		return err
	}

	const queryRecent = `
		SELECT EXISTS (
			SELECT 1
			FROM otp_verificaciones
			WHERE usuario_id = $1
				AND tipo = $2
				AND created_at > now() - make_interval(secs => $3)
		)
	`

	var recent bool
	if err := tx.QueryRow(ctx, queryRecent, userID, passwordResetOTPType, cooldown.Seconds()).Scan(&recent); err != nil {
		return err
	}
	if recent {
		return ErrResetCodeTooSoon
	}

	const queryReplace = `
		UPDATE otp_verificaciones
		SET estado = 'reemplazado'
		WHERE usuario_id = $1
			AND tipo = $2
			AND estado = 'pendiente'
	`
	if _, err := tx.Exec(ctx, queryReplace, userID, passwordResetOTPType); err != nil {
		return err
	}

	const queryInsert = `
		INSERT INTO otp_verificaciones (usuario_id, codigo_hash, tipo, estado, expira_at)
		VALUES ($1, $2, $3, 'pendiente', now() + make_interval(secs => $4))
	`
	if _, err := tx.Exec(ctx, queryInsert, userID, codeHash, passwordResetOTPType, ttl.Seconds()); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DiscardPasswordResetCode borra los códigos pendientes de userID. Se usa cuando el
// correo con el código no se pudo enviar, para que el usuario pueda pedir otro sin
// esperar.
func DiscardPasswordResetCode(pool *pgxpool.Pool, userID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const query = `
		DELETE FROM otp_verificaciones
		WHERE usuario_id = $1
			AND tipo = $2
			AND estado = 'pendiente'
	`
	_, err := pool.Exec(ctx, query, userID, passwordResetOTPType)
	return err
}

// ResetPasswordWithCode cambia el password_hash de userID por newPasswordHash si code
// coincide con su código de recuperación vigente, y marca el código como usado.
//
// Cada código incorrecto suma un intento; al llegar a [MaxPasswordResetAttempts] el
// código se bloquea. En cualquier caso de código no válido regresa
// [ErrInvalidResetCode].
func ResetPasswordWithCode(pool *pgxpool.Pool, userID uuid.UUID, code, newPasswordHash string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const queryCurrent = `
		SELECT id, codigo_hash, intentos
		FROM otp_verificaciones
		WHERE usuario_id = $1
			AND tipo = $2
			AND estado = 'pendiente'
			AND expira_at > now()
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`

	var otpID uuid.UUID
	var codeHash string
	var attempts int
	err = tx.QueryRow(ctx, queryCurrent, userID, passwordResetOTPType).Scan(&otpID, &codeHash, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidResetCode
	}
	if err != nil {
		return err
	}

	if bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(code)) != nil {
		const queryFail = `
			UPDATE otp_verificaciones
			SET intentos = intentos + 1,
				estado = CASE WHEN intentos + 1 >= $2 THEN 'bloqueado' ELSE estado END
			WHERE id = $1
		`
		if _, err := tx.Exec(ctx, queryFail, otpID, MaxPasswordResetAttempts); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return ErrInvalidResetCode
	}

	if _, err := tx.Exec(ctx, `UPDATE usuarios SET password_hash = $1 WHERE id = $2`, newPasswordHash, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `UPDATE otp_verificaciones SET estado = 'usado' WHERE id = $1`, otpID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
